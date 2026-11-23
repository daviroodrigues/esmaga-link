package service

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"time"

	"github.com/daviroodrigues/esmaga-api/internal/domain"
	"github.com/daviroodrigues/esmaga-api/internal/idgen"
	"github.com/daviroodrigues/esmaga-api/internal/queue/rabbitmq"
	"github.com/daviroodrigues/esmaga-api/internal/repository/cassandra"
	"github.com/daviroodrigues/esmaga-api/internal/repository/redis"
	"github.com/daviroodrigues/esmaga-api/internal/security"
	"github.com/google/uuid"
)

var (
	ErrLinkNotFound     = errors.New("link não encontrado ou inativo")
	ErrLinkImmutable    = errors.New("links anônimos são estritamente imutáveis")
	ErrUnauthorizedLink = errors.New("você não tem permissão para gerenciar este link")
	ErrInvalidURL       = errors.New("a URL informada é inválida")
)

type LinkService struct {
	idGen       *idgen.IDGenerator
	linkRepo    *cassandra.LinkRepository
	cacheRepo   *redis.CacheRepository
	publisher   *rabbitmq.Publisher
	authService *AuthService
	validator   *security.URLValidator
}

func NewLinkService(
	idGen *idgen.IDGenerator,
	linkRepo *cassandra.LinkRepository,
	cacheRepo *redis.CacheRepository,
	publisher *rabbitmq.Publisher,
	authService *AuthService,
	validator *security.URLValidator,
) *LinkService {
	return &LinkService{
		idGen:       idGen,
		linkRepo:    linkRepo,
		cacheRepo:   cacheRepo,
		publisher:   publisher,
		authService: authService,
		validator:   validator,
	}
}

func (s *LinkService) CreateLink(ctx context.Context, originalURL string, userID *uuid.UUID) (*domain.Link, error) {
	// Proteção contra SSRF e verificação de integridade da URL
	if s.validator != nil {
		if err := s.validator.ValidateDestination(originalURL); err != nil {
			return nil, err
		}
	} else {
		parsed, err := url.ParseRequestURI(originalURL)
		if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") {
			return nil, ErrInvalidURL
		}
	}

	// Geração com tolerância a colisões (IF NOT EXISTS no Cassandra)
	var link *domain.Link
	maxAttempts := 3

	for attempt := 0; attempt < maxAttempts; attempt++ {
		shortCode, err := s.idGen.NextID(ctx)
		if err != nil {
			return nil, fmt.Errorf("erro ao gerar short code: %w", err)
		}

		link = &domain.Link{
			ShortCode:   shortCode,
			OriginalURL: originalURL,
			UserID:      userID,
			IsActive:    true,
			CreatedAt:   time.Now().UTC(),
		}

		err = s.linkRepo.Create(ctx, link)
		if err == nil {
			break
		}

		if errors.Is(err, cassandra.ErrCodeCollision) {
			// Colisão detectada pelo Cassandra (LWT Paxos), tenta o próximo ID
			if attempt == maxAttempts-1 {
				return nil, errors.New("limite de tentativas excedido ao resolver colisão de link")
			}
			continue
		}

		return nil, fmt.Errorf("erro ao salvar link: %w", err)
	}

	// Pré-aquece o cache no Redis (24 horas)
	_ = s.cacheRepo.SetLink(ctx, link.ShortCode, originalURL, 24*time.Hour)

	return link, nil
}

// GetLinkByCode implementa o caminho crítico (SLO < 15ms):
// 1. Tenta Redis Cache
// 2. Se miss, busca Cassandra e popula Redis
func (s *LinkService) GetLinkByCode(ctx context.Context, shortCode string) (string, error) {
	cachedURL, err := s.cacheRepo.GetLink(ctx, shortCode)
	if err == nil && cachedURL != "" {
		return cachedURL, nil
	}

	// Cache Miss -> Cassandra
	link, err := s.linkRepo.GetByCode(ctx, shortCode)
	if err != nil {
		return "", err
	}
	if link == nil || !link.IsActive {
		return "", ErrLinkNotFound
	}

	// Popula cache para os próximos acessos
	_ = s.cacheRepo.SetLink(ctx, shortCode, link.OriginalURL, 24*time.Hour)

	return link.OriginalURL, nil
}

func (s *LinkService) EditLink(ctx context.Context, shortCode string, userID uuid.UUID, newURL string) error {
	if s.validator != nil {
		if err := s.validator.ValidateDestination(newURL); err != nil {
			return err
		}
	} else {
		parsed, err := url.ParseRequestURI(newURL)
		if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") {
			return ErrInvalidURL
		}
	}

	link, err := s.linkRepo.GetByCode(ctx, shortCode)
	if err != nil {
		return err
	}
	if link == nil {
		return ErrLinkNotFound
	}

	// Links anônimos são imutáveis
	if link.UserID == nil {
		return ErrLinkImmutable
	}

	// Apenas o dono pode editar
	if *link.UserID != userID {
		return ErrUnauthorizedLink
	}

	// Atualiza Cassandra
	if err := s.linkRepo.UpdateURL(ctx, shortCode, userID, newURL); err != nil {
		return err
	}

	// Invalida cache no Redis
	_ = s.cacheRepo.DeleteLink(ctx, shortCode)

	return nil
}

func (s *LinkService) GetLinkAnalytics(ctx context.Context, shortCode string, userID uuid.UUID, days int) (*domain.LinkAnalytics, error) {
	link, err := s.linkRepo.GetByCode(ctx, shortCode)
	if err != nil {
		return nil, err
	}
	if link == nil {
		return nil, ErrLinkNotFound
	}

	if link.UserID == nil || *link.UserID != userID {
		return nil, ErrUnauthorizedLink
	}

	return s.linkRepo.GetAnalyticsByCode(ctx, shortCode, days)
}

func (s *LinkService) ListUserLinks(ctx context.Context, userID uuid.UUID) ([]*domain.Link, error) {
	return s.linkRepo.GetByUser(ctx, userID)
}

func (s *LinkService) DeleteAccount(ctx context.Context, userID uuid.UUID, keepLinksActive bool) error {
	if keepLinksActive {
		// Boa prática: Torna os links anônimos e imutáveis para não quebrar a web
		if err := s.linkRepo.AnonymizeUserLinks(ctx, userID); err != nil {
			return err
		}
	} else {
		// Desativa e invalida cache
		codes, err := s.linkRepo.DeactivateUserLinks(ctx, userID)
		if err != nil {
			return err
		}
		for _, code := range codes {
			_ = s.cacheRepo.DeleteLink(ctx, code)
		}
	}

	// Deleta usuário do PostgreSQL
	return s.authService.DeleteUser(ctx, userID)
}

func (s *LinkService) TrackClickAsync(event *domain.ClickEvent) {
	if s.publisher == nil {
		return
	}
	// Disparo assíncrono em goroutine para não atrasar a resposta 302
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = s.publisher.PublishClick(ctx, event)
	}()
}
