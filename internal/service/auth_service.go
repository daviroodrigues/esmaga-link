package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/daviroodrigues/esmaga-api/internal/domain"
	"github.com/daviroodrigues/esmaga-api/internal/queue/rabbitmq"
	"github.com/daviroodrigues/esmaga-api/internal/repository/postgres"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

var (
	ErrEmailAlreadyExists     = errors.New("email já cadastrado")
	ErrInvalidCredentials     = errors.New("credenciais inválidas")
	ErrEmailNotVerified       = errors.New("e-mail ainda não verificado")
	ErrAccountSuspended       = errors.New("conta suspensa")
	ErrEmailAlreadyVerified   = errors.New("e-mail já verificado")
	ErrInvalidAuthToken       = errors.New("token inválido ou expirado")
	ErrInvalidRefreshToken    = errors.New("refresh token inválido ou expirado")
	ErrInvalidCurrentPassword = errors.New("senha atual incorreta")
)

const (
	accessTokenTTL   = 15 * time.Minute
	refreshTokenTTL  = 30 * 24 * time.Hour
	emailVerifyTTL   = 24 * time.Hour
	passwordResetTTL = 30 * time.Minute
)

type AuthService struct {
	userRepo   *postgres.UserRepository
	tokenRepo  *postgres.TokenRepository
	publisher  *rabbitmq.Publisher
	jwtSecret  []byte
	appBaseURL string
}

type TokenClaims struct {
	UserID        uuid.UUID `json:"user_id"`
	Email         string    `json:"email"`
	Role          string    `json:"role"`
	EmailVerified bool      `json:"email_verified"`
	jwt.RegisteredClaims
}

type AuthTokens struct {
	AccessToken   string `json:"access_token" example:"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."`
	RefreshToken  string `json:"refresh_token" example:"dGhpcy1pcy1hLXJlZnJlc2gtdG9rZW4..."`
	TokenType     string `json:"token_type" example:"Bearer"`
	ExpiresIn     int64  `json:"expires_in" example:"900"`
	EmailVerified bool   `json:"email_verified" example:"true"`
}

func NewAuthService(
	userRepo *postgres.UserRepository,
	tokenRepo *postgres.TokenRepository,
	publisher *rabbitmq.Publisher,
	jwtSecret string,
	appBaseURL string,
) *AuthService {
	return &AuthService{
		userRepo:   userRepo,
		tokenRepo:  tokenRepo,
		publisher:  publisher,
		jwtSecret:  []byte(jwtSecret),
		appBaseURL: strings.TrimRight(appBaseURL, "/"),
	}
}

func (s *AuthService) Register(ctx context.Context, email, password string, name *string) (*domain.User, error) {
	email = strings.TrimSpace(strings.ToLower(email))
	existing, err := s.userRepo.GetByEmail(ctx, email)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return nil, ErrEmailAlreadyExists
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("erro ao gerar hash da senha: %w", err)
	}

	user := &domain.User{
		Name:         name,
		Email:        email,
		PasswordHash: string(hash),
		Role:         "user",
		Status:       domain.UserStatusPending,
	}

	if err := s.userRepo.Create(ctx, user); err != nil {
		return nil, err
	}

	if err := s.issueAndSendVerification(ctx, user); err != nil {
		return nil, err
	}

	return user, nil
}

func (s *AuthService) VerifyEmail(ctx context.Context, rawToken string) error {
	if s.tokenRepo == nil {
		return ErrInvalidAuthToken
	}

	tok, err := s.tokenRepo.ConsumeAuthToken(ctx, hashToken(rawToken), domain.TokenPurposeEmailVerification)
	if err != nil {
		return err
	}
	if tok == nil {
		return ErrInvalidAuthToken
	}

	return s.userRepo.MarkEmailVerified(ctx, tok.UserID)
}

func (s *AuthService) ResendVerification(ctx context.Context, email string) error {
	email = strings.TrimSpace(strings.ToLower(email))
	user, err := s.userRepo.GetByEmail(ctx, email)
	if err != nil {
		return err
	}
	// Evita enumeração de contas inexistentes
	if user == nil {
		return nil
	}
	if user.EmailVerifiedAt != nil {
		return ErrEmailAlreadyVerified
	}

	return s.issueAndSendVerification(ctx, user)
}

func (s *AuthService) Login(ctx context.Context, email, password, userAgent, ipAddress string) (*AuthTokens, error) {
	email = strings.TrimSpace(strings.ToLower(email))
	user, err := s.userRepo.GetByEmail(ctx, email)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, ErrInvalidCredentials
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return nil, ErrInvalidCredentials
	}

	if user.Status == domain.UserStatusSuspended {
		return nil, ErrAccountSuspended
	}
	if user.EmailVerifiedAt == nil {
		return nil, ErrEmailNotVerified
	}

	_ = s.userRepo.UpdateLastLogin(ctx, user.ID)
	return s.issueTokenPair(ctx, user, userAgent, ipAddress)
}

func (s *AuthService) Refresh(ctx context.Context, rawRefreshToken, userAgent, ipAddress string) (*AuthTokens, error) {
	if s.tokenRepo == nil || rawRefreshToken == "" {
		return nil, ErrInvalidRefreshToken
	}

	tokenHash := hashToken(rawRefreshToken)
	stored, err := s.tokenRepo.GetActiveRefreshToken(ctx, tokenHash)
	if err != nil {
		return nil, err
	}
	if stored == nil {
		return nil, ErrInvalidRefreshToken
	}

	if err := s.tokenRepo.RevokeRefreshToken(ctx, tokenHash); err != nil {
		return nil, err
	}

	user, err := s.userRepo.GetByID(ctx, stored.UserID)
	if err != nil {
		return nil, err
	}
	if user == nil || user.Status == domain.UserStatusSuspended {
		return nil, ErrInvalidRefreshToken
	}

	return s.issueTokenPair(ctx, user, userAgent, ipAddress)
}

func (s *AuthService) Logout(ctx context.Context, rawRefreshToken string) error {
	if s.tokenRepo == nil || rawRefreshToken == "" {
		return nil
	}
	return s.tokenRepo.RevokeRefreshToken(ctx, hashToken(rawRefreshToken))
}

func (s *AuthService) ForgotPassword(ctx context.Context, email string) error {
	email = strings.TrimSpace(strings.ToLower(email))
	user, err := s.userRepo.GetByEmail(ctx, email)
	if err != nil {
		return err
	}
	// Sempre retorna sucesso para evitar enumeração de e-mails
	if user == nil || user.Status == domain.UserStatusSuspended {
		return nil
	}

	if s.tokenRepo != nil {
		_ = s.tokenRepo.InvalidatePendingAuthTokens(ctx, user.ID, domain.TokenPurposePasswordReset)

		rawToken, err := generateOpaqueToken()
		if err != nil {
			return err
		}

		err = s.tokenRepo.CreateAuthToken(ctx, &domain.AuthToken{
			UserID:    user.ID,
			TokenHash: hashToken(rawToken),
			Purpose:   domain.TokenPurposePasswordReset,
			ExpiresAt: time.Now().UTC().Add(passwordResetTTL),
		})
		if err != nil {
			return err
		}

		s.publishEmail(ctx, rabbitmq.RoutingKeyPasswordReset, &domain.EmailMessage{
			To:       user.Email,
			Template: domain.TemplatePasswordReset,
			Variables: map[string]string{
				"action_url": fmt.Sprintf("%s/reset-password?token=%s", s.appBaseURL, rawToken),
			},
			IdempotencyKey: uuid.NewString(),
		})
	}

	return nil
}

func (s *AuthService) ResetPassword(ctx context.Context, rawToken, newPassword string) error {
	if s.tokenRepo == nil {
		return ErrInvalidAuthToken
	}

	tok, err := s.tokenRepo.ConsumeAuthToken(ctx, hashToken(rawToken), domain.TokenPurposePasswordReset)
	if err != nil {
		return err
	}
	if tok == nil {
		return ErrInvalidAuthToken
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("erro ao gerar hash da senha: %w", err)
	}

	if err := s.userRepo.UpdatePassword(ctx, tok.UserID, string(hash)); err != nil {
		return err
	}

	return s.tokenRepo.RevokeAllUserRefreshTokens(ctx, tok.UserID)
}

func (s *AuthService) GetProfile(ctx context.Context, userID uuid.UUID) (*domain.User, error) {
	return s.userRepo.GetByID(ctx, userID)
}

func (s *AuthService) UpdateProfile(ctx context.Context, userID uuid.UUID, name *string, currentPassword, newPassword string) (*domain.User, error) {
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, ErrInvalidCredentials
	}

	if newPassword != "" {
		if currentPassword == "" {
			return nil, ErrInvalidCurrentPassword
		}
		if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(currentPassword)); err != nil {
			return nil, ErrInvalidCurrentPassword
		}
		hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
		if err != nil {
			return nil, fmt.Errorf("erro ao gerar hash da senha: %w", err)
		}
		if err := s.userRepo.UpdatePassword(ctx, userID, string(hash)); err != nil {
			return nil, err
		}
	}

	if name != nil {
		trimmed := strings.TrimSpace(*name)
		if err := s.userRepo.UpdateName(ctx, userID, &trimmed); err != nil {
			return nil, err
		}
	}

	return s.userRepo.GetByID(ctx, userID)
}

func (s *AuthService) IsEmailVerified(ctx context.Context, userID uuid.UUID) (bool, error) {
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return false, err
	}
	return user != nil && user.EmailVerifiedAt != nil, nil
}

func (s *AuthService) ValidateToken(tokenStr string) (*TokenClaims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &TokenClaims{}, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("método de assinatura inesperado: %v", t.Header["alg"])
		}
		return s.jwtSecret, nil
	})

	if err != nil || !token.Valid {
		return nil, errors.New("token inválido ou expirado")
	}

	claims, ok := token.Claims.(*TokenClaims)
	if !ok {
		return nil, errors.New("falha ao ler claims do token")
	}

	return claims, nil
}

func (s *AuthService) DeleteUser(ctx context.Context, userID uuid.UUID) error {
	user, _ := s.userRepo.GetByID(ctx, userID)
	if err := s.userRepo.Delete(ctx, userID); err != nil {
		return err
	}
	if user != nil {
		s.publishEmail(ctx, rabbitmq.RoutingKeyAccountDeleted, &domain.EmailMessage{
			To:             user.Email,
			Template:       domain.TemplateAccountDeleted,
			Variables:      map[string]string{},
			IdempotencyKey: uuid.NewString(),
		})
	}
	return nil
}

func (s *AuthService) issueAndSendVerification(ctx context.Context, user *domain.User) error {
	if s.tokenRepo == nil {
		return nil
	}
	_ = s.tokenRepo.InvalidatePendingAuthTokens(ctx, user.ID, domain.TokenPurposeEmailVerification)

	rawToken, err := generateOpaqueToken()
	if err != nil {
		return err
	}

	err = s.tokenRepo.CreateAuthToken(ctx, &domain.AuthToken{
		UserID:    user.ID,
		TokenHash: hashToken(rawToken),
		Purpose:   domain.TokenPurposeEmailVerification,
		ExpiresAt: time.Now().UTC().Add(emailVerifyTTL),
	})
	if err != nil {
		return err
	}

	name := ""
	if user.Name != nil {
		name = *user.Name
	}

	s.publishEmail(ctx, rabbitmq.RoutingKeyEmailVerification, &domain.EmailMessage{
		To:       user.Email,
		Template: domain.TemplateEmailVerification,
		Variables: map[string]string{
			"name":       name,
			"action_url": fmt.Sprintf("%s/verify-email?token=%s", s.appBaseURL, rawToken),
		},
		IdempotencyKey: uuid.NewString(),
	})

	return nil
}

func (s *AuthService) issueTokenPair(ctx context.Context, user *domain.User, userAgent, ipAddress string) (*AuthTokens, error) {
	verified := user.EmailVerifiedAt != nil
	claims := TokenClaims{
		UserID:        user.ID,
		Email:         user.Email,
		Role:          user.Role,
		EmailVerified: verified,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(accessTokenTTL)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	jwtToken := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	accessStr, err := jwtToken.SignedString(s.jwtSecret)
	if err != nil {
		return nil, fmt.Errorf("erro ao assinar access token: %w", err)
	}

	var refreshStr string
	if s.tokenRepo != nil {
		refreshStr, err = generateOpaqueToken()
		if err != nil {
			return nil, err
		}
		err = s.tokenRepo.CreateRefreshToken(ctx, &domain.RefreshToken{
			UserID:    user.ID,
			TokenHash: hashToken(refreshStr),
			UserAgent: userAgent,
			IPAddress: ipAddress,
			ExpiresAt: time.Now().UTC().Add(refreshTokenTTL),
		})
		if err != nil {
			return nil, err
		}
	}

	return &AuthTokens{
		AccessToken:   accessStr,
		RefreshToken:  refreshStr,
		TokenType:     "Bearer",
		ExpiresIn:     int64(accessTokenTTL.Seconds()),
		EmailVerified: verified,
	}, nil
}

func (s *AuthService) publishEmail(ctx context.Context, routingKey string, msg *domain.EmailMessage) {
	if s.publisher == nil {
		slog.Warn("publisher RabbitMQ indisponível; e-mail não enfileirado",
			slog.String("routing_key", routingKey),
			slog.String("to", msg.To),
		)
		return
	}
	pubCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	if err := s.publisher.PublishEmail(pubCtx, routingKey, msg); err != nil {
		slog.Error("falha ao enfileirar e-mail no RabbitMQ",
			slog.String("routing_key", routingKey),
			slog.String("to", msg.To),
			slog.String("error", err.Error()),
		)
		return
	}
	slog.Info("e-mail enfileirado no RabbitMQ",
		slog.String("routing_key", routingKey),
		slog.String("to", msg.To),
	)
}

func generateOpaqueToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("erro ao gerar token aleatório: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func hashToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}
