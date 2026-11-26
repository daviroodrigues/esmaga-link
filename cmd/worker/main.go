package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"esmaga-link/internal/config"
	"esmaga-link/internal/domain"
	"esmaga-link/internal/email"
	"esmaga-link/internal/observability"
	"esmaga-link/internal/queue/rabbitmq"
	cassRepo "esmaga-link/internal/repository/cassandra"
	pgRepo "esmaga-link/internal/repository/postgres"
	"github.com/gocql/gocql"
	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	cfg := config.Load()
	observability.InitLogger("esmaga-worker", cfg.InstanceID)
	log.Println("[Worker] Inicializando consumidores (analytics + e-mail)...")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	pgPool, err := pgxpool.New(ctx, cfg.PostgresURL)
	if err != nil {
		log.Fatalf("[Worker] Falha ao conectar no PostgreSQL: %v", err)
	}
	defer pgPool.Close()

	templateRepo := pgRepo.NewEmailTemplateRepository(pgPool)
	defaults, err := email.DefaultTemplates()
	if err != nil {
		log.Fatalf("[Worker] Falha ao carregar templates padrão: %v", err)
	}
	if err := templateRepo.SeedDefaults(ctx, defaults); err != nil {
		log.Fatalf("[Worker] Falha ao semear templates de e-mail no PostgreSQL: %v", err)
	}
	log.Println("[Worker] Templates de e-mail sincronizados com o PostgreSQL.")

	cassHosts := strings.Split(cfg.CassandraHosts, ",")
	cluster := gocql.NewCluster(cassHosts...)
	cluster.Keyspace = cfg.CassandraKeyspace
	cluster.Consistency = gocql.Quorum
	cluster.ConnectTimeout = 10 * time.Second
	cluster.Timeout = 5 * time.Second

	cassSession, err := cluster.CreateSession()
	if err != nil {
		log.Fatalf("[Worker] Falha ao conectar no Cassandra: %v", err)
	}
	defer cassSession.Close()
	log.Println("[Worker] Conectado ao Cassandra com sucesso.")

	linkRepo := cassRepo.NewLinkRepository(cassSession)

	clickConsumer, err := rabbitmq.NewConsumer(cfg.RabbitMQURL)
	if err != nil {
		log.Fatalf("[Worker] Falha ao conectar consumidor de cliques RabbitMQ: %v", err)
	}
	defer clickConsumer.Close()

	emailConsumer, err := rabbitmq.NewEmailConsumer(cfg.RabbitMQURL)
	if err != nil {
		log.Fatalf("[Worker] Falha ao conectar consumidor de e-mail RabbitMQ: %v", err)
	}
	defer emailConsumer.Close()
	log.Println("[Worker] Conectado às filas do RabbitMQ.")

	emailRenderer := email.NewRenderer(templateRepo)
	emailSender, err := email.NewSender(cfg)
	if err != nil {
		log.Fatalf("[Worker] Configuração de e-mail inválida: %v", err)
	}

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-sigChan
		log.Println("[Worker] Encerrando worker...")
		cancel()
		clickConsumer.Close()
		emailConsumer.Close()
		cassSession.Close()
		pgPool.Close()
		os.Exit(0)
	}()

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		err := clickConsumer.Consume(func(event *domain.ClickEvent) error {
			reqCtx, reqCancel := context.WithTimeout(ctx, 3*time.Second)
			defer reqCancel()

			if err := linkRepo.InsertClickEvent(reqCtx, event); err != nil {
				log.Printf("[Analytics Worker] Falha ao persistir clique no Cassandra: %v", err)
				return err
			}

			log.Printf("[Analytics Worker] Clique registrado para short_code=%s (ip=%s, device=%s)",
				event.ShortCode, event.IPAddress, event.DeviceType)
			return nil
		})
		if err != nil {
			log.Printf("[Analytics Worker] Erro durante consumo da fila: %v", err)
		}
	}()

	go func() {
		defer wg.Done()
		err := emailConsumer.Consume(func(msg *domain.EmailMessage) error {
			reqCtx, reqCancel := context.WithTimeout(ctx, 15*time.Second)
			defer reqCancel()

			subj, htmlBody, textBody, err := emailRenderer.Render(reqCtx, msg.Template, msg.Variables)
			if err != nil {
				return err
			}

			if err := emailSender.Send(reqCtx, msg.To, subj, htmlBody, textBody); err != nil {
				return err
			}

			log.Printf("[Email Worker] E-mail %q enviado para %s", msg.Template, msg.To)
			return nil
		})
		if err != nil {
			log.Printf("[Email Worker] Erro durante consumo da fila: %v", err)
		}
	}()

	wg.Wait()
}
