package http

import (
	"encoding/json"
	"net/http"

	"github.com/daviroodrigues/esmaga-api/internal/domain"
	"github.com/daviroodrigues/esmaga-api/internal/service"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type LinkHandler struct {
	linkService *service.LinkService
}

func NewLinkHandler(linkService *service.LinkService) *LinkHandler {
	return &LinkHandler{linkService: linkService}
}

type EditLinkResponse struct {
	Message     string `json:"message" example:"link atualizado com sucesso"`
	ShortCode   string `json:"short_code" example:"aB3x9Z"`
	OriginalURL string `json:"original_url" example:"https://exemplo.com/novo-destino"`
}

// Create godoc
// @Summary		Encurtar URL
// @Description	Cria um novo link encurtado. Aceita requisições anônimas ou autenticadas via Bearer token.
// @Tags			Links
// @Accept			json
// @Produce		json
// @Security		BearerAuth
// @Param			request	body		domain.CreateLinkRequest	true	"URL original para encurtar"
// @Success		201		{object}	domain.Link
// @Failure		400		{object}	ErrorResponse
// @Router			/api/v1/links [post]
func (h *LinkHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req domain.CreateLinkRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.OriginalURL == "" {
		http.Error(w, `{"error": "campo original_url é obrigatório"}`, http.StatusBadRequest)
		return
	}

	var userID *uuid.UUID
	if claims, ok := r.Context().Value(UserClaimsKey).(*service.TokenClaims); ok && claims != nil {
		userID = &claims.UserID
	}

	link, err := h.linkService.CreateLink(r.Context(), req.OriginalURL, userID)
	if err != nil {
		http.Error(w, `{"error": "`+err.Error()+`"}`, http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(link)
}

// Edit godoc
// @Summary		Editar URL de destino de um link
// @Description	Atualiza a URL original de um link pertencente ao usuário autenticado
// @Tags			Links
// @Accept			json
// @Produce		json
// @Security		BearerAuth
// @Param			code	path		string					true	"Código curto do link"
// @Param			request	body		domain.EditLinkRequest	true	"Nova URL original"
// @Success		200		{object}	EditLinkResponse
// @Failure		400		{object}	ErrorResponse
// @Failure		401		{object}	ErrorResponse
// @Failure		403		{object}	ErrorResponse
// @Failure		404		{object}	ErrorResponse
// @Failure		409		{object}	ErrorResponse
// @Router			/api/v1/links/{code} [put]
func (h *LinkHandler) Edit(w http.ResponseWriter, r *http.Request) {
	claims, ok := r.Context().Value(UserClaimsKey).(*service.TokenClaims)
	if !ok || claims == nil {
		http.Error(w, `{"error": "não autorizado"}`, http.StatusUnauthorized)
		return
	}

	code := chi.URLParam(r, "code")
	if code == "" {
		http.Error(w, `{"error": "código do link não informado"}`, http.StatusBadRequest)
		return
	}

	var req domain.EditLinkRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.OriginalURL == "" {
		http.Error(w, `{"error": "campo original_url é obrigatório"}`, http.StatusBadRequest)
		return
	}

	if err := h.linkService.EditLink(r.Context(), code, claims.UserID, req.OriginalURL); err != nil {
		status := http.StatusBadRequest
		if err == service.ErrUnauthorizedLink {
			status = http.StatusForbidden
		} else if err == service.ErrLinkImmutable {
			status = http.StatusConflict
		} else if err == service.ErrLinkNotFound {
			status = http.StatusNotFound
		}
		http.Error(w, `{"error": "`+err.Error()+`"}`, status)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{
		"message":      "link atualizado com sucesso",
		"short_code":   code,
		"original_url": req.OriginalURL,
	})
}

// ListMyLinks godoc
// @Summary		Listar links do usuário
// @Description	Retorna todos os links criados pelo usuário autenticado
// @Tags			Links
// @Produce		json
// @Security		BearerAuth
// @Success		200	{array}		domain.Link
// @Failure		401	{object}	ErrorResponse
// @Failure		500	{object}	ErrorResponse
// @Router			/api/v1/links [get]
func (h *LinkHandler) ListMyLinks(w http.ResponseWriter, r *http.Request) {
	claims, ok := r.Context().Value(UserClaimsKey).(*service.TokenClaims)
	if !ok || claims == nil {
		http.Error(w, `{"error": "não autorizado"}`, http.StatusUnauthorized)
		return
	}

	links, err := h.linkService.ListUserLinks(r.Context(), claims.UserID)
	if err != nil {
		http.Error(w, `{"error": "`+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}

	if links == nil {
		links = []*domain.Link{}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(links)
}
