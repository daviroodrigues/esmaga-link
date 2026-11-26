package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/daviroodrigues/esmaga-api/internal/config"
	httpHandler "github.com/daviroodrigues/esmaga-api/internal/handler/http"
	"github.com/daviroodrigues/esmaga-api/internal/idgen"
	"github.com/daviroodrigues/esmaga-api/internal/observability"
	"github.com/daviroodrigues/esmaga-api/internal/queue/rabbitmq"
	cassRepo "github.com/daviroodrigues/esmaga-api/internal/repository/cassandra"
	pgRepo "github.com/daviroodrigues/esmaga-api/internal/repository/postgres"
	redisRepo "github.com/daviroodrigues/esmaga-api/internal/repository/redis"
	"github.com/daviroodrigues/esmaga-api/internal/security"
	"github.com/daviroodrigues/esmaga-api/internal/service"
	"github.com/gocql/gocql"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

// @title						Esmaga Link API
// @version					1.0
// @description				API de encurtamento de URLs de alta performance com redirecionamento rápido e rastreamento assíncrono de cliques.
// @BasePath					/
// @securityDefinitions.apikey	BearerAuth
// @in							header
// @name						Authorization
// @description				Informe o token JWT no formato: Bearer {token}
func main() {
	cfg := config.Load()
	observability.InitLogger("esmaga-api", cfg.InstanceID)
	metrics := observability.NewMetrics()
	log.Printf("[%s] Iniciando Esmaga Link API na porta %s...", cfg.InstanceID, cfg.Port)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	pgPool, err := pgxpool.New(ctx, cfg.PostgresURL)
	if err != nil {
		log.Fatalf("Falha ao conectar no PostgreSQL: %v", err)
	}
	defer pgPool.Close()

	if err := pgPool.Ping(ctx); err != nil {
		log.Printf("Aviso: Ping no PostgreSQL falhou: %v", err)
	} else {
		log.Printf("[%s] Conectado ao PostgreSQL com sucesso.", cfg.InstanceID)
	}

	cassHosts := strings.Split(cfg.CassandraHosts, ",")
	cluster := gocql.NewCluster(cassHosts...)
	cluster.Keyspace = cfg.CassandraKeyspace
	cluster.Consistency = gocql.Quorum
	cluster.ConnectTimeout = 10 * time.Second
	cluster.Timeout = 5 * time.Second

	cassSession, err := cluster.CreateSession()
	if err != nil {
		log.Fatalf("Falha ao conectar no Cassandra: %v", err)
	}
	defer cassSession.Close()
	log.Printf("[%s] Conectado ao Cassandra com sucesso.", cfg.InstanceID)

	rdb := redis.NewClient(&redis.Options{
		Addr:         cfg.RedisAddr,
		Password:     cfg.RedisPassword,
		PoolSize:     64,
		MinIdleConns: 16,
	})
	defer rdb.Close()
	if err := rdb.Ping(ctx).Err(); err != nil {
		log.Fatalf("Falha ao conectar no Redis: %v", err)
	}
	log.Printf("[%s] Conectado ao Redis com sucesso.", cfg.InstanceID)

	publisher, err := rabbitmq.NewPublisher(cfg.RabbitMQURL)
	if err != nil {
		log.Printf("Aviso: Falha ao conectar RabbitMQ Publisher (%v). Ingestão assíncrona desabilitada.", err)
	} else {
		defer publisher.Close()
		log.Printf("[%s] Conectado ao RabbitMQ Publisher.", cfg.InstanceID)
	}

	userRepository := pgRepo.NewUserRepository(pgPool)
	tokenRepository := pgRepo.NewTokenRepository(pgPool)
	linkRepository := cassRepo.NewLinkRepository(cassSession)
	cacheRepository := redisRepo.NewCacheRepository(rdb)
	urlValidator := security.NewURLValidator(nil)

	base62Enc, err := idgen.NewBase62EncoderWithSeed(cfg.AlphabetSeed, 5)
	if err != nil {
		log.Fatalf("Falha ao inicializar Base62 encoder: %v", err)
	}

	eras, err := idgen.DefaultEras()
	if err != nil {
		log.Fatalf("Falha ao inicializar Eras: %v", err)
	}

	idGenerator, err := idgen.NewMultiEraIDGenerator(rdb, "esmaga:counter:id_sequence", cfg.IDBatchSize, eras, base62Enc, linkRepository)
	if err != nil {
		log.Fatalf("Falha ao criar IDGenerator: %v", err)
	}

	if err := idGenerator.InitOrRecover(ctx); err != nil {
		log.Printf("Aviso: Falha ao executar InitOrRecover do IDGenerator: %v", err)
	} else {
		log.Printf("[%s] Motor de IDs sincronizado com sucesso com o Cassandra (SSOT).", cfg.InstanceID)
	}

	authService := service.NewAuthService(userRepository, tokenRepository, publisher, cfg.JWTSecret, cfg.AppBaseURL)
	linkService := service.NewLinkService(idGenerator, linkRepository, cacheRepository, publisher, authService, urlValidator)
	go linkService.StartCacheInvalidationListener(ctx)

	turnstileVerifier := security.NewTurnstileVerifier(cfg.TurnstileSecretKey)

	authHandler := httpHandler.NewAuthHandler(authService, linkService, turnstileVerifier)
	linkHandler := httpHandler.NewLinkHandler(linkService, turnstileVerifier)
	redirectHandler := httpHandler.NewRedirectHandler(linkService)
	rateLimiter := httpHandler.NewRateLimiter(rdb)

	router := httpHandler.NewRouter(cfg.InstanceID, cfg.TurnstileSiteKey, cfg.PortfolioMode, metrics, authService, authHandler, linkHandler, redirectHandler, rateLimiter)

	srv := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      router,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
		<-sigChan

		log.Printf("[%s] Encerrando servidor graciosamente...", cfg.InstanceID)
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer shutdownCancel()

		if err := srv.Shutdown(shutdownCtx); err != nil {
			log.Printf("Erro no encerramento: %v", err)
		}
	}()

	log.Printf("[%s] Servidor pronto para atender requisições em :%s", cfg.InstanceID, cfg.Port)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatalf("Erro fatal no servidor HTTP: %v", err)
	}
}
