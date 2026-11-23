package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/daviroodrigues/esmaga-api/internal/config"
	"github.com/daviroodrigues/esmaga-api/internal/domain"
	"github.com/daviroodrigues/esmaga-api/internal/queue/rabbitmq"
	cassRepo "github.com/daviroodrigues/esmaga-api/internal/repository/cassandra"
	"github.com/gocql/gocql"
)

func main() {
	cfg := config.Load()
	log.Println("[Analytics Worker] Inicializando consumidor de métricas...")

	// 1. Conexão com Cassandra
	cassHosts := strings.Split(cfg.CassandraHosts, ",")
	cluster := gocql.NewCluster(cassHosts...)
	cluster.Keyspace = cfg.CassandraKeyspace
	cluster.Consistency = gocql.Quorum
	cluster.ConnectTimeout = 10 * time.Second
	cluster.Timeout = 5 * time.Second

	cassSession, err := cluster.CreateSession()
	if err != nil {
		log.Fatalf("[Analytics Worker] Falha ao conectar no Cassandra: %v", err)
	}
	defer cassSession.Close()
	log.Println("[Analytics Worker] Conectado ao Cassandra com sucesso.")

	linkRepo := cassRepo.NewLinkRepository(cassSession)

	// 2. Conexão com RabbitMQ Consumer
	consumer, err := rabbitmq.NewConsumer(cfg.RabbitMQURL)
	if err != nil {
		log.Fatalf("[Analytics Worker] Falha ao conectar consumidor RabbitMQ: %v", err)
	}
	defer consumer.Close()
	log.Println("[Analytics Worker] Conectado à fila do RabbitMQ.")

	// Graceful Shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-sigChan
		log.Println("[Analytics Worker] Encerrando worker...")
		consumer.Close()
		cassSession.Close()
		os.Exit(0)
	}()

	// 3. Loop de consumo de eventos de clique
	err = consumer.Consume(func(event *domain.ClickEvent) error {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()

		if err := linkRepo.InsertClickEvent(ctx, event); err != nil {
			log.Printf("[Analytics Worker] Falha ao persistir clique no Cassandra: %v", err)
			return err
		}

		log.Printf("[Analytics Worker] Clique registrado para short_code=%s (ip=%s, device=%s)",
			event.ShortCode, event.IPAddress, event.DeviceType)
		return nil
	})

	if err != nil {
		log.Fatalf("[Analytics Worker] Erro durante consumo da fila: %v", err)
	}
}
