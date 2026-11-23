package http

import (
	"net/http"
	"strings"
	"time"

	"github.com/daviroodrigues/esmaga-api/internal/domain"
	"github.com/daviroodrigues/esmaga-api/internal/service"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type RedirectHandler struct {
	linkService *service.LinkService
}

func NewRedirectHandler(linkService *service.LinkService) *RedirectHandler {
	return &RedirectHandler{linkService: linkService}
}

// HandleRedirect godoc
// @Summary		Redirecionar para URL original
// @Description	Redireciona (HTTP 302) para a URL original associada ao código curto e registra o clique de forma assíncrona
// @Tags			Redirect
// @Param			code	path	string	true	"Código curto do link"
// @Success		302		{string}	string	"Redirecionamento para a URL original"
// @Failure		404		{string}	string	"Link não encontrado"
// @Router			/r/{code} [get]
func (h *RedirectHandler) HandleRedirect(w http.ResponseWriter, r *http.Request) {
	code := chi.URLParam(r, "code")
	if code == "" {
		http.NotFound(w, r)
		return
	}

	originalURL, err := h.linkService.GetLinkByCode(r.Context(), code)
	if err != nil || originalURL == "" {
		http.NotFound(w, r)
		return
	}

	// Disparo de métricas assíncrono (não bloqueia a resposta do usuário)
	event := &domain.ClickEvent{
		EventID:    uuid.New(),
		ShortCode:  code,
		ClickedAt:  time.Now().UTC(),
		IPAddress:  getRealIP(r),
		UserAgent:  r.UserAgent(),
		Referrer:   r.Referer(),
		Country:    r.Header.Get("CF-IPCountry"), // Se atrás de CDN ou proxy
		DeviceType: detectDevice(r.UserAgent()),
	}
	h.linkService.TrackClickAsync(event)

	// Resposta 302 imediata sem alocação de corpo HTML
	w.Header().Set("Location", originalURL)
	w.Header().Set("Cache-Control", "private, max-age=90")
	w.WriteHeader(http.StatusFound)
}

func getRealIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		return strings.TrimSpace(parts[0])
	}
	if xrip := r.Header.Get("X-Real-IP"); xrip != "" {
		return strings.TrimSpace(xrip)
	}
	return r.RemoteAddr
}

func detectDevice(ua string) string {
	uaLower := strings.ToLower(ua)
	if strings.Contains(uaLower, "mobile") || strings.Contains(uaLower, "android") || strings.Contains(uaLower, "iphone") {
		return "mobile"
	}
	if strings.Contains(uaLower, "tablet") || strings.Contains(uaLower, "ipad") {
		return "tablet"
	}
	return "desktop"
}
