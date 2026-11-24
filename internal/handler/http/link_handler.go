package http

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

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
		writeJSONError(w, http.StatusBadRequest, "campo original_url é obrigatório")
		return
	}

	var userID *uuid.UUID
	if claims, ok := r.Context().Value(UserClaimsKey).(*service.TokenClaims); ok && claims != nil {
		userID = &claims.UserID
	}

	link, err := h.linkService.CreateLink(r.Context(), req.OriginalURL, userID)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, link)
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
		writeJSONError(w, http.StatusUnauthorized, "não autorizado")
		return
	}

	code := chi.URLParam(r, "code")
	if code == "" {
		writeJSONError(w, http.StatusBadRequest, "código do link não informado")
		return
	}

	var req domain.EditLinkRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.OriginalURL == "" {
		writeJSONError(w, http.StatusBadRequest, "campo original_url é obrigatório")
		return
	}

	if err := h.linkService.EditLink(r.Context(), code, claims.UserID, req.OriginalURL); err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, service.ErrUnauthorizedLink) {
			status = http.StatusForbidden
		} else if errors.Is(err, service.ErrLinkImmutable) {
			status = http.StatusConflict
		} else if errors.Is(err, service.ErrLinkNotFound) {
			status = http.StatusNotFound
		}
		writeJSONError(w, status, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, EditLinkResponse{
		Message:     "link atualizado com sucesso",
		ShortCode:   code,
		OriginalURL: req.OriginalURL,
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
		writeJSONError(w, http.StatusUnauthorized, "não autorizado")
		return
	}

	links, err := h.linkService.ListUserLinks(r.Context(), claims.UserID)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if links == nil {
		links = []*domain.Link{}
	}

	writeJSON(w, http.StatusOK, links)
}

// GetStats godoc
// @Summary		Estatísticas de cliques de um link
// @Description	Retorna métricas agregadas de cliques (por data, dispositivo, país e referrer) de um link do usuário autenticado
// @Tags			Links
// @Produce		json
// @Security		BearerAuth
// @Param			code	path		string	true	"Código curto do link"
// @Param			days	query		int		false	"Quantidade de dias para agregação (1 a 90, padrão 7)"
// @Success		200		{object}	domain.LinkAnalytics
// @Failure		400		{object}	ErrorResponse
// @Failure		401		{object}	ErrorResponse
// @Failure		403		{object}	ErrorResponse
// @Failure		404		{object}	ErrorResponse
// @Failure		500		{object}	ErrorResponse
// @Router			/api/v1/links/{code}/stats [get]
func (h *LinkHandler) GetStats(w http.ResponseWriter, r *http.Request) {
	claims, ok := r.Context().Value(UserClaimsKey).(*service.TokenClaims)
	if !ok || claims == nil {
		writeJSONError(w, http.StatusUnauthorized, "não autorizado")
		return
	}

	code := chi.URLParam(r, "code")
	if code == "" {
		writeJSONError(w, http.StatusBadRequest, "código do link não informado")
		return
	}

	days := 7
	if rawDays := r.URL.Query().Get("days"); rawDays != "" {
		parsedDays, err := strconv.Atoi(rawDays)
		if err != nil || parsedDays < 1 || parsedDays > 90 {
			writeJSONError(w, http.StatusBadRequest, "parâmetro days deve ser um inteiro entre 1 e 90")
			return
		}
		days = parsedDays
	}

	stats, err := h.linkService.GetLinkAnalytics(r.Context(), code, claims.UserID, days)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, service.ErrLinkNotFound) {
			status = http.StatusNotFound
		} else if errors.Is(err, service.ErrUnauthorizedLink) {
			status = http.StatusForbidden
		}
		writeJSONError(w, status, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, stats)
}
