package http

import (
	"encoding/json"
	"net/http"

	_ "github.com/daviroodrigues/esmaga-api/docs"
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

func NewRouter(
	instanceID string,
	authService *service.AuthService,
	authHandler *AuthHandler,
	linkHandler *LinkHandler,
	redirectHandler *RedirectHandler,
) http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.Recoverer)

	// Hot Path: redirecionamento 302 com overhead mínimo (sem Logger síncrono nem CORS)
	r.Group(func(r chi.Router) {
		r.Use(middleware.RealIP)
		r.Get("/r/{code}", redirectHandler.HandleRedirect)
	})

	// Demais rotas com middlewares completos
	r.Group(func(r chi.Router) {
		r.Use(middleware.RequestID)
		r.Use(middleware.RealIP)
		r.Use(middleware.Logger)
		r.Use(cors.Handler(cors.Options{
			AllowedOrigins:   []string{"*"},
			AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
			AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
			ExposedHeaders:   []string{"Link"},
			AllowCredentials: true,
			MaxAge:           300,
		}))

		// Health check
		r.Get("/healthz", healthzHandler(instanceID))

		// Documentação Swagger
		r.Get("/swagger/*", httpSwagger.WrapHandler)

		// API v1
		r.Route("/api/v1", func(r chi.Router) {
			// Rotas de Autenticação
			r.Route("/auth", func(r chi.Router) {
				r.Post("/register", authHandler.Register)
				r.Post("/login", authHandler.Login)
				r.With(AuthRequiredMiddleware(authService)).Delete("/account", authHandler.DeleteAccount)
			})

			// Rotas de Links
			r.Route("/links", func(r chi.Router) {
				// Criação aceita usuário anônimo ou autenticado
				r.With(OptionalAuthMiddleware(authService)).Post("/", linkHandler.Create)

				// Rotas autenticadas
				r.Group(func(r chi.Router) {
					r.Use(AuthRequiredMiddleware(authService))
					r.Get("/", linkHandler.ListMyLinks)
					r.Put("/{code}", linkHandler.Edit)
				})
			})
		})
	})

	return r
}
