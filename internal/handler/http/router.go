package http

import (
	"encoding/json"
	"net/http"

	_ "github.com/daviroodrigues/esmaga-api/docs"
	"github.com/daviroodrigues/esmaga-api/internal/observability"
	"github.com/daviroodrigues/esmaga-api/internal/service"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	httpSwagger "github.com/swaggo/http-swagger/v2"
)

type ErrorResponse struct {
	Error string `json:"error" example:"mensagem de erro"`
}

type HealthResponse struct {
	Status   string `json:"status" example:"ok"`
	Instance string `json:"instance" example:"api-1"`
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeJSONError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, ErrorResponse{Error: msg})
}

// healthzHandler godoc
// @Summary		Health check da API
// @Description	Retorna o status da instância da API
// @Tags			Health
// @Produce		json
// @Success		200	{object}	HealthResponse
// @Router			/healthz [get]
func healthzHandler(instanceID string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(HealthResponse{
			Status:   "ok",
			Instance: instanceID,
		})
	}
}

type PublicConfigResponse struct {
	TurnstileSiteKey string `json:"turnstile_site_key" example:""`
}

func NewRouter(
	instanceID string,
	turnstileSiteKey string,
	metrics *observability.Metrics,
	authService *service.AuthService,
	authHandler *AuthHandler,
	linkHandler *LinkHandler,
	redirectHandler *RedirectHandler,
	rateLimiter *RateLimiter,
) http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.Recoverer)

	r.Group(func(r chi.Router) {
		r.Use(middleware.RealIP)
		r.Get("/r/{code}", func(w http.ResponseWriter, req *http.Request) {
			if metrics != nil {
				metrics.IncRedirect()
			}
			redirectHandler.HandleRedirect(w, req)
		})
	})

	r.Group(func(r chi.Router) {
		r.Use(middleware.RequestID)
		r.Use(middleware.RealIP)
		if metrics != nil {
			r.Use(metrics.HTTPMiddleware)
		} else {
			r.Use(middleware.Logger)
		}
		r.Use(cors.Handler(cors.Options{
			AllowedOrigins:   []string{"*"},
			AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
			AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
			ExposedHeaders:   []string{"Link"},
			AllowCredentials: true,
			MaxAge:           300,
		}))

		r.Get("/healthz", healthzHandler(instanceID))
		if metrics != nil {
			r.Get("/metrics", metrics.Handler())
		}

		r.Get("/swagger/*", httpSwagger.WrapHandler)

		r.Route("/api/v1", func(r chi.Router) {
			r.Get("/config", func(w http.ResponseWriter, r *http.Request) {
				writeJSON(w, http.StatusOK, PublicConfigResponse{
					TurnstileSiteKey: turnstileSiteKey,
				})
			})

			r.Route("/auth", func(r chi.Router) {
				r.Group(func(r chi.Router) {
					if rateLimiter != nil {
						r.Use(rateLimiter.Limit("auth", 20, 60e9))
					}
					r.Post("/register", authHandler.Register)
					r.Post("/verify-email", authHandler.VerifyEmail)
					r.Post("/login", authHandler.Login)
					r.Post("/refresh", authHandler.Refresh)
					r.Post("/logout", authHandler.Logout)
					r.Post("/forgot-password", authHandler.ForgotPassword)
					r.Post("/reset-password", authHandler.ResetPassword)
				})
				r.Group(func(r chi.Router) {
					if rateLimiter != nil {
						r.Use(rateLimiter.Limit("resend_verification", 1, 60e9))
					}
					r.Post("/resend-verification", authHandler.ResendVerification)
				})
				r.Group(func(r chi.Router) {
					r.Use(AuthRequiredMiddleware(authService))
					r.Get("/me", authHandler.GetMe)
					r.Patch("/me", authHandler.UpdateMe)
					r.Delete("/account", authHandler.DeleteAccount)
				})
			})

			r.Route("/links", func(r chi.Router) {

				r.Group(func(r chi.Router) {
					r.Use(OptionalAuthMiddleware(authService))
					if rateLimiter != nil {
						r.Use(rateLimiter.Limit("create_link", 30, 60e9))
					}
					r.Post("/", linkHandler.Create)
				})

				r.Group(func(r chi.Router) {
					r.Use(AuthRequiredMiddleware(authService))
					r.Get("/", linkHandler.ListMyLinks)
					r.Get("/{code}/stats", linkHandler.GetStats)
					r.Put("/{code}", linkHandler.Edit)
					r.Delete("/{code}", linkHandler.Delete)
				})
			})
		})
	})

	return r
}
