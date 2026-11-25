package http

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/daviroodrigues/esmaga-api/internal/domain"
	"github.com/daviroodrigues/esmaga-api/internal/security"
	"github.com/daviroodrigues/esmaga-api/internal/service"
)

const refreshTokenCookieName = "refresh_token"

type AuthHandler struct {
	authService *service.AuthService
	linkService *service.LinkService
	turnstile   *security.TurnstileVerifier
}

func NewAuthHandler(authService *service.AuthService, linkService *service.LinkService, turnstile *security.TurnstileVerifier) *AuthHandler {
	return &AuthHandler{authService: authService, linkService: linkService, turnstile: turnstile}
}

type RegisterRequest struct {
	Name           *string `json:"name,omitempty" example:"Davi"`
	Email          string  `json:"email" example:"usuario@exemplo.com"`
	Password       string  `json:"password" example:"senha123"`
	TurnstileToken string  `json:"turnstile_token,omitempty" example:""`
}

type AuthRequest struct {
	Email    string `json:"email" example:"usuario@exemplo.com"`
	Password string `json:"password" example:"senha123"`
}

type TokenRequest struct {
	Token string `json:"token" example:"token-de-verificacao"`
}

type EmailRequest struct {
	Email          string `json:"email" example:"usuario@exemplo.com"`
	TurnstileToken string `json:"turnstile_token,omitempty" example:""`
}

type RefreshRequest struct {
	RefreshToken string `json:"refresh_token,omitempty" example:"refresh-token-aqui"`
}

type ResetPasswordRequest struct {
	Token       string `json:"token" example:"token-de-reset"`
	NewPassword string `json:"new_password" example:"novaSenha123"`
}

type UpdateProfileRequest struct {
	Name            *string `json:"name,omitempty" example:"Davi Rodrigues"`
	CurrentPassword string  `json:"current_password,omitempty" example:"senhaAtual123"`
	NewPassword     string  `json:"new_password,omitempty" example:"novaSenha123"`
}

type MessageResponse struct {
	Message string `json:"message" example:"operação realizada com sucesso"`
}

type DeleteAccountResponse struct {
	Message         string `json:"message" example:"conta excluída com sucesso"`
	KeepLinksActive bool   `json:"keep_links_active" example:"true"`
}

// Register godoc
// @Summary		Cadastrar novo usuário
// @Description	Cria conta com status pendente e dispara e-mail de verificação assíncrono
// @Tags			Auth
// @Accept			json
// @Produce		json
// @Param			request	body		RegisterRequest	true	"Dados de cadastro"
// @Success		201		{object}	domain.User
// @Failure		400		{object}	ErrorResponse
// @Router			/api/v1/auth/register [post]
func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	var req RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Email == "" || len(req.Password) < 6 {
		writeJSONError(w, http.StatusBadRequest, "dados inválidos, senha deve ter ao menos 6 caracteres")
		return
	}

	if h.turnstile != nil {
		if err := h.turnstile.Verify(r.Context(), req.TurnstileToken, r.RemoteAddr); err != nil {
			writeJSONError(w, http.StatusBadRequest, err.Error())
			return
		}
	}

	user, err := h.authService.Register(r.Context(), req.Email, req.Password, req.Name)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, user)
}

// VerifyEmail godoc
// @Summary		Confirmar e-mail
// @Description	Valida o token de verificação de uso único e ativa a conta
// @Tags			Auth
// @Accept			json
// @Produce		json
// @Param			request	body		TokenRequest	true	"Token de verificação"
// @Success		200		{object}	MessageResponse
// @Failure		400		{object}	ErrorResponse
// @Router			/api/v1/auth/verify-email [post]
func (h *AuthHandler) VerifyEmail(w http.ResponseWriter, r *http.Request) {
	var req TokenRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Token == "" {
		writeJSONError(w, http.StatusBadRequest, "token é obrigatório")
		return
	}

	if err := h.authService.VerifyEmail(r.Context(), req.Token); err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, MessageResponse{Message: "e-mail verificado com sucesso"})
}

// ResendVerification godoc
// @Summary		Reenviar verificação de e-mail
// @Description	Reenvia o e-mail de confirmação para contas pendentes (rate limit: 1 req/min)
// @Tags			Auth
// @Accept			json
// @Produce		json
// @Param			request	body		EmailRequest	true	"E-mail cadastrado"
// @Success		200		{object}	MessageResponse
// @Failure		400		{object}	ErrorResponse
// @Router			/api/v1/auth/resend-verification [post]
func (h *AuthHandler) ResendVerification(w http.ResponseWriter, r *http.Request) {
	var req EmailRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Email == "" {
		writeJSONError(w, http.StatusBadRequest, "e-mail é obrigatório")
		return
	}

	if err := h.authService.ResendVerification(r.Context(), req.Email); err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, MessageResponse{Message: "se o e-mail existir e estiver pendente, enviaremos uma nova confirmação"})
}

// Login godoc
// @Summary		Autenticar usuário
// @Description	Autentica o usuário e retorna par access_token (15min) + refresh_token (30 dias)
// @Tags			Auth
// @Accept			json
// @Produce		json
// @Param			request	body		AuthRequest			true	"Credenciais de acesso"
// @Success		200		{object}	service.AuthTokens
// @Failure		400		{object}	ErrorResponse
// @Failure		401		{object}	ErrorResponse
// @Failure		403		{object}	ErrorResponse
// @Router			/api/v1/auth/login [post]
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req AuthRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "requisição malformatada")
		return
	}

	tokens, err := h.authService.Login(r.Context(), req.Email, req.Password, r.UserAgent(), r.RemoteAddr)
	if err != nil {
		status := http.StatusUnauthorized
		if errors.Is(err, service.ErrEmailNotVerified) || errors.Is(err, service.ErrAccountSuspended) {
			status = http.StatusForbidden
		}
		writeJSONError(w, status, err.Error())
		return
	}

	setRefreshCookie(w, tokens.RefreshToken)
	writeJSON(w, http.StatusOK, tokens)
}

// Refresh godoc
// @Summary		Renovar sessão
// @Description	Rotaciona o refresh_token (via body ou cookie HttpOnly) e emite um novo par de tokens
// @Tags			Auth
// @Accept			json
// @Produce		json
// @Param			request	body		RefreshRequest		false	"Refresh token (opcional se enviado via cookie)"
// @Success		200		{object}	service.AuthTokens
// @Failure		401		{object}	ErrorResponse
// @Router			/api/v1/auth/refresh [post]
func (h *AuthHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	rawToken := extractRefreshToken(r)
	if rawToken == "" {
		writeJSONError(w, http.StatusUnauthorized, "refresh token não informado")
		return
	}

	tokens, err := h.authService.Refresh(r.Context(), rawToken, r.UserAgent(), r.RemoteAddr)
	if err != nil {
		writeJSONError(w, http.StatusUnauthorized, err.Error())
		return
	}

	setRefreshCookie(w, tokens.RefreshToken)
	writeJSON(w, http.StatusOK, tokens)
}

// Logout godoc
// @Summary		Encerrar sessão
// @Description	Revoga o refresh_token atual e limpa o cookie de sessão
// @Tags			Auth
// @Accept			json
// @Produce		json
// @Param			request	body		RefreshRequest	false	"Refresh token (opcional se enviado via cookie)"
// @Success		200		{object}	MessageResponse
// @Router			/api/v1/auth/logout [post]
func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	rawToken := extractRefreshToken(r)
	_ = h.authService.Logout(r.Context(), rawToken)
	clearRefreshCookie(w)
	writeJSON(w, http.StatusOK, MessageResponse{Message: "sessão encerrada com sucesso"})
}

// ForgotPassword godoc
// @Summary		Solicitar redefinição de senha
// @Description	Responde sempre 200 OK e, se o e-mail existir, envia link de redefinição (TTL 30min)
// @Tags			Auth
// @Accept			json
// @Produce		json
// @Param			request	body		EmailRequest	true	"E-mail cadastrado"
// @Success		200		{object}	MessageResponse
// @Failure		400		{object}	ErrorResponse
// @Router			/api/v1/auth/forgot-password [post]
func (h *AuthHandler) ForgotPassword(w http.ResponseWriter, r *http.Request) {
	var req EmailRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Email == "" {
		writeJSONError(w, http.StatusBadRequest, "e-mail é obrigatório")
		return
	}

	if h.turnstile != nil {
		if err := h.turnstile.Verify(r.Context(), req.TurnstileToken, r.RemoteAddr); err != nil {
			writeJSONError(w, http.StatusBadRequest, err.Error())
			return
		}
	}

	_ = h.authService.ForgotPassword(r.Context(), req.Email)
	writeJSON(w, http.StatusOK, MessageResponse{Message: "se o e-mail existir, enviaremos instruções para redefinição de senha"})
}

// ResetPassword godoc
// @Summary		Redefinir senha
// @Description	Valida o token de redefinição, atualiza a senha e revoga sessões ativas
// @Tags			Auth
// @Accept			json
// @Produce		json
// @Param			request	body		ResetPasswordRequest	true	"Token e nova senha"
// @Success		200		{object}	MessageResponse
// @Failure		400		{object}	ErrorResponse
// @Router			/api/v1/auth/reset-password [post]
func (h *AuthHandler) ResetPassword(w http.ResponseWriter, r *http.Request) {
	var req ResetPasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Token == "" || len(req.NewPassword) < 6 {
		writeJSONError(w, http.StatusBadRequest, "token e nova senha (mínimo 6 caracteres) são obrigatórios")
		return
	}

	if err := h.authService.ResetPassword(r.Context(), req.Token, req.NewPassword); err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, MessageResponse{Message: "senha redefinida com sucesso"})
}

// GetMe godoc
// @Summary		Perfil do usuário
// @Description	Retorna os dados da conta autenticada
// @Tags			Auth
// @Produce		json
// @Security		BearerAuth
// @Success		200	{object}	domain.User
// @Failure		401	{object}	ErrorResponse
// @Router			/api/v1/auth/me [get]
func (h *AuthHandler) GetMe(w http.ResponseWriter, r *http.Request) {
	claims, ok := r.Context().Value(UserClaimsKey).(*service.TokenClaims)
	if !ok || claims == nil {
		writeJSONError(w, http.StatusUnauthorized, "não autorizado")
		return
	}

	user, err := h.authService.GetProfile(r.Context(), claims.UserID)
	if err != nil || user == nil {
		writeJSONError(w, http.StatusNotFound, "usuário não encontrado")
		return
	}

	writeJSON(w, http.StatusOK, user)
}

// UpdateMe godoc
// @Summary		Atualizar perfil ou senha
// @Description	Atualiza nome do usuário e/ou altera a senha (exigindo current_password)
// @Tags			Auth
// @Accept			json
// @Produce		json
// @Security		BearerAuth
// @Param			request	body		UpdateProfileRequest	true	"Campos para atualização"
// @Success		200		{object}	domain.User
// @Failure		400		{object}	ErrorResponse
// @Failure		401		{object}	ErrorResponse
// @Router			/api/v1/auth/me [patch]
func (h *AuthHandler) UpdateMe(w http.ResponseWriter, r *http.Request) {
	claims, ok := r.Context().Value(UserClaimsKey).(*service.TokenClaims)
	if !ok || claims == nil {
		writeJSONError(w, http.StatusUnauthorized, "não autorizado")
		return
	}

	var req UpdateProfileRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "requisição malformatada")
		return
	}
	if req.NewPassword != "" && len(req.NewPassword) < 6 {
		writeJSONError(w, http.StatusBadRequest, "nova senha deve ter ao menos 6 caracteres")
		return
	}

	user, err := h.authService.UpdateProfile(r.Context(), claims.UserID, req.Name, req.CurrentPassword, req.NewPassword)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, user)
}

// DeleteAccount godoc
// @Summary		Excluir conta do usuário
// @Description	Exclui a conta do usuário autenticado, permitindo escolher se os links permanecem ativos
// @Tags			Auth
// @Accept			json
// @Produce		json
// @Security		BearerAuth
// @Param			request	body		domain.DeleteAccountRequest	false	"Opções de exclusão"
// @Success		200		{object}	DeleteAccountResponse
// @Failure		401		{object}	ErrorResponse
// @Failure		500		{object}	ErrorResponse
// @Router			/api/v1/auth/account [delete]
func (h *AuthHandler) DeleteAccount(w http.ResponseWriter, r *http.Request) {
	claims, ok := r.Context().Value(UserClaimsKey).(*service.TokenClaims)
	if !ok || claims == nil {
		writeJSONError(w, http.StatusUnauthorized, "não autorizado")
		return
	}

	var req domain.DeleteAccountRequest
	// Default: manter links anônimos se o campo não for explicitamente enviado como falso
	req.KeepLinksActive = true
	_ = json.NewDecoder(r.Body).Decode(&req)

	if err := h.linkService.DeleteAccount(r.Context(), claims.UserID, req.KeepLinksActive); err != nil {
		writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}

	clearRefreshCookie(w)
	writeJSON(w, http.StatusOK, DeleteAccountResponse{
		Message:         "conta excluída com sucesso",
		KeepLinksActive: req.KeepLinksActive,
	})
}

func extractRefreshToken(r *http.Request) string {
	var req RefreshRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err == nil && req.RefreshToken != "" {
		return req.RefreshToken
	}
	if cookie, err := r.Cookie(refreshTokenCookieName); err == nil {
		return cookie.Value
	}
	return ""
}

func setRefreshCookie(w http.ResponseWriter, token string) {
	if token == "" {
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     refreshTokenCookieName,
		Value:    token,
		Path:     "/api/v1/auth",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int((30 * 24 * time.Hour).Seconds()),
	})
}

func clearRefreshCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     refreshTokenCookieName,
		Value:    "",
		Path:     "/api/v1/auth",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
}
