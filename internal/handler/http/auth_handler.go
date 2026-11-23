package http

import (
	"encoding/json"
	"net/http"

	"github.com/daviroodrigues/esmaga-api/internal/domain"
	"github.com/daviroodrigues/esmaga-api/internal/service"
)

type AuthHandler struct {
	authService *service.AuthService
	linkService *service.LinkService
}

func NewAuthHandler(authService *service.AuthService, linkService *service.LinkService) *AuthHandler {
	return &AuthHandler{authService: authService, linkService: linkService}
}

type AuthRequest struct {
	Email    string `json:"email" example:"usuario@exemplo.com"`
	Password string `json:"password" example:"senha123"`
}

type LoginResponse struct {
	Token string `json:"token" example:"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."`
}

type DeleteAccountResponse struct {
	Message         string `json:"message" example:"conta excluída com sucesso"`
	KeepLinksActive bool   `json:"keep_links_active" example:"true"`
}

// Register godoc
// @Summary		Cadastrar novo usuário
// @Description	Cria uma nova conta de usuário com e-mail e senha (mínimo 6 caracteres)
// @Tags			Auth
// @Accept			json
// @Produce		json
// @Param			request	body		AuthRequest		true	"Dados de cadastro"
// @Success		201		{object}	domain.User
// @Failure		400		{object}	ErrorResponse
// @Router			/api/v1/auth/register [post]
func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	var req AuthRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Email == "" || len(req.Password) < 6 {
		http.Error(w, `{"error": "dados inválidos, senha deve ter ao menos 6 caracteres"}`, http.StatusBadRequest)
		return
	}

	user, err := h.authService.Register(r.Context(), req.Email, req.Password)
	if err != nil {
		http.Error(w, `{"error": "`+err.Error()+`"}`, http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(user)
}

// Login godoc
// @Summary		Autenticar usuário
// @Description	Autentica o usuário e retorna um token JWT
// @Tags			Auth
// @Accept			json
// @Produce		json
// @Param			request	body		AuthRequest		true	"Credenciais de acesso"
// @Success		200		{object}	LoginResponse
// @Failure		400		{object}	ErrorResponse
// @Failure		401		{object}	ErrorResponse
// @Router			/api/v1/auth/login [post]
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req AuthRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error": "requisição malformatada"}`, http.StatusBadRequest)
		return
	}

	token, err := h.authService.Login(r.Context(), req.Email, req.Password)
	if err != nil {
		http.Error(w, `{"error": "`+err.Error()+`"}`, http.StatusUnauthorized)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"token": token})
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
		http.Error(w, `{"error": "não autorizado"}`, http.StatusUnauthorized)
		return
	}

	var req domain.DeleteAccountRequest
	// Default: manter links anônimos se o campo não for explicitamente enviado como falso
	req.KeepLinksActive = true
	_ = json.NewDecoder(r.Body).Decode(&req)

	if err := h.linkService.DeleteAccount(r.Context(), claims.UserID, req.KeepLinksActive); err != nil {
		http.Error(w, `{"error": "`+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"message":           "conta excluída com sucesso",
		"keep_links_active": req.KeepLinksActive,
	})
}
