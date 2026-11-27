package service

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"time"

	"esmaga-link/internal/domain"
	"esmaga-link/internal/idgen"
	"esmaga-link/internal/queue/rabbitmq"
	"esmaga-link/internal/repository/cassandra"
	"esmaga-link/internal/repository/redis"
	"esmaga-link/internal/security"
	"github.com/google/uuid"
)

var (
	ErrLinkNotFound     = errors.New("link não encontrado ou inativo")
	ErrLinkImmutable    = errors.New("links anônimos são estritamente imutáveis")
	ErrUnauthorizedLink = errors.New("você não tem permissão para gerenciar este link")
	ErrInvalidURL       = errors.New("a URL informada é inválida")
)

const (
	l1CacheMaxEntries = 50000
	l1CacheTTL        = 30 * time.Second
	redisCacheTTL     = 24 * time.Hour
)

type LinkService struct {
	idGen       *idgen.IDGenerator
	linkRepo    *cassandra.LinkRepository
	cacheRepo   *redis.CacheRepository
	l1Cache     *l1Cache
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
		l1Cache:     newL1Cache(l1CacheMaxEntries),
		publisher:   publisher,
		authService: authService,
		validator:   validator,
	}
}

func (s *LinkService) CreateLink(ctx context.Context, originalURL string, userID *uuid.UUID) (*domain.Link, error) {

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

			if attempt == maxAttempts-1 {
				return nil, errors.New("limite de tentativas excedido ao resolver colisão de link")
			}
			continue
		}

		return nil, fmt.Errorf("erro ao salvar link: %w", err)
	}

	if s.l1Cache != nil {
		s.l1Cache.Set(link.ShortCode, originalURL, l1CacheTTL)
	}
	_ = s.cacheRepo.SetLink(ctx, link.ShortCode, originalURL, redisCacheTTL)

	return link, nil
}

func (s *LinkService) GetLinkByCode(ctx context.Context, shortCode string) (string, error) {
	if s.l1Cache != nil {
		if cachedURL, ok := s.l1Cache.Get(shortCode); ok {
			return cachedURL, nil
		}
	}

	cachedURL, err := s.cacheRepo.GetLink(ctx, shortCode)
	if err == nil && cachedURL != "" {
		if s.l1Cache != nil {
			s.l1Cache.Set(shortCode, cachedURL, l1CacheTTL)
		}
		return cachedURL, nil
	}

	link, err := s.linkRepo.GetByCode(ctx, shortCode)
	if err != nil {
		return "", err
	}
	if link == nil || !link.IsActive {
		return "", ErrLinkNotFound
	}

	if s.l1Cache != nil {
		s.l1Cache.Set(shortCode, link.OriginalURL, l1CacheTTL)
	}

	go func(code, targetURL string) {
		bgCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = s.cacheRepo.SetLink(bgCtx, code, targetURL, redisCacheTTL)
	}(shortCode, link.OriginalURL)

	return link.OriginalURL, nil
}

func (s *LinkService) EditLink(ctx context.Context, shortCode string, userID uuid.UUID, newURL string) error {
	if s.authService != nil {
		verified, err := s.authService.IsEmailVerified(ctx, userID)
		if err != nil {
			return err
		}
		if !verified {
			return ErrEmailNotVerified
		}
	}

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

	if link.UserID == nil {
		return ErrLinkImmutable
	}

	if *link.UserID != userID {
		return ErrUnauthorizedLink
	}

	if err := s.linkRepo.UpdateURL(ctx, shortCode, userID, newURL); err != nil {
		return err
	}

	s.invalidateCache(ctx, shortCode)

	return nil
}

func (s *LinkService) SetLinkStatus(ctx context.Context, shortCode string, userID uuid.UUID, isActive bool) error {
	if s.authService != nil {
		verified, err := s.authService.IsEmailVerified(ctx, userID)
		if err != nil {
			return err
		}
		if !verified {
			return ErrEmailNotVerified
		}
	}

	link, err := s.linkRepo.GetByCode(ctx, shortCode)
	if err != nil {
		return err
	}
	if link == nil || (link.UserID == nil && !link.IsActive) {
		return ErrLinkNotFound
	}
	if link.UserID == nil {
		return ErrLinkImmutable
	}
	if *link.UserID != userID {
		return ErrUnauthorizedLink
	}

	if link.IsActive == isActive {
		return nil
	}

	if err := s.linkRepo.SetLinkStatus(ctx, shortCode, userID, link.CreatedAt, isActive); err != nil {
		return err
	}

	if isActive {
		if s.l1Cache != nil {
			s.l1Cache.Set(shortCode, link.OriginalURL, l1CacheTTL)
		}
		if s.cacheRepo != nil {
			_ = s.cacheRepo.SetLink(ctx, shortCode, link.OriginalURL, redisCacheTTL)
		}
	} else {
		s.invalidateCache(ctx, shortCode)
	}

	return nil
}

func (s *LinkService) DeleteLink(ctx context.Context, shortCode string, userID uuid.UUID) error {
	if s.authService != nil {
		verified, err := s.authService.IsEmailVerified(ctx, userID)
		if err != nil {
			return err
		}
		if !verified {
			return ErrEmailNotVerified
		}
	}

	link, err := s.linkRepo.GetByCode(ctx, shortCode)
	if err != nil {
		return err
	}
	if link == nil || (link.UserID == nil && !link.IsActive) {
		return ErrLinkNotFound
	}
	if link.UserID == nil {
		return ErrLinkImmutable
	}
	if *link.UserID != userID {
		return ErrUnauthorizedLink
	}

	if err := s.linkRepo.PermanentDeleteLink(ctx, shortCode, userID, link.CreatedAt); err != nil {
		return err
	}

	s.invalidateCache(ctx, shortCode)
	return nil
}

func (s *LinkService) StartCacheInvalidationListener(ctx context.Context) {
	if s.cacheRepo == nil || s.l1Cache == nil {
		return
	}
	s.cacheRepo.SubscribeInvalidations(ctx, func(shortCode string) {
		s.l1Cache.Delete(shortCode)
	})
}

func (s *LinkService) invalidateCache(ctx context.Context, shortCode string) {
	if s.l1Cache != nil {
		s.l1Cache.Delete(shortCode)
	}
	if s.cacheRepo != nil {
		_ = s.cacheRepo.DeleteLink(ctx, shortCode)
		_ = s.cacheRepo.PublishInvalidation(ctx, shortCode)
	}
}

func (s *LinkService) GetLinkAnalytics(ctx context.Context, shortCode string, userID uuid.UUID, days int, loc *time.Location) (*domain.LinkAnalytics, error) {
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

	return s.linkRepo.GetAnalyticsByCode(ctx, shortCode, days, loc)
}

func (s *LinkService) ListUserLinks(ctx context.Context, userID uuid.UUID) ([]*domain.Link, error) {
	return s.linkRepo.GetByUser(ctx, userID)
}

func (s *LinkService) DeleteAccount(ctx context.Context, userID uuid.UUID, keepLinksActive bool) error {
	if keepLinksActive {

		if err := s.linkRepo.AnonymizeUserLinks(ctx, userID); err != nil {
			return err
		}
	} else {

		codes, err := s.linkRepo.DeactivateUserLinks(ctx, userID)
		if err != nil {
			return err
		}
		for _, code := range codes {
			s.invalidateCache(ctx, code)
		}
	}

	return s.authService.DeleteUser(ctx, userID)
}

func (s *LinkService) TrackClickAsync(event *domain.ClickEvent) {
	if s.publisher == nil {
		return
	}
	s.publisher.EnqueueClick(event)
}
