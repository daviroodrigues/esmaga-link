package rabbitmq

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/daviroodrigues/esmaga-api/internal/domain"
	amqp "github.com/rabbitmq/amqp091-go"
)

const (
	ExchangeClicks      = "clicks.topic"
	QueueClicks         = "clicks.cassandra.writer"
	RoutingKey          = "click.created"
	publishBufferSize   = 4096
)

type Publisher struct {
	conn      *amqp.Connection
	channel   *amqp.Channel
	pubMu     sync.Mutex
	events    chan *domain.ClickEvent
	done      chan struct{}
	closeOnce sync.Once
}

func NewPublisher(url string) (*Publisher, error) {
	conn, err := amqp.Dial(url)
	if err != nil {
		return nil, fmt.Errorf("erro ao conectar no rabbitmq: %w", err)
	}

	ch, err := conn.Channel()
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("erro ao abrir canal rabbitmq: %w", err)
	}

	// Declara a exchange de cliques
	err = ch.ExchangeDeclare(
		ExchangeClicks,
		"topic",
		true,  // durable
		false, // auto-deleted
		false, // internal
		false, // no-wait
		nil,
	)
	if err != nil {
		ch.Close()
		conn.Close()
		return nil, fmt.Errorf("erro ao declarar exchange: %w", err)
	}

	if err := declareNotificationsTopology(ch); err != nil {
		ch.Close()
		conn.Close()
		return nil, err
	}

	p := &Publisher{
		conn:    conn,
		channel: ch,
		events:  make(chan *domain.ClickEvent, publishBufferSize),
		done:    make(chan struct{}),
	}
	go p.publishLoop()

	return p, nil
}

func (p *Publisher) publishLoop() {
	defer close(p.done)
	for event := range p.events {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		_ = p.PublishClick(ctx, event)
		cancel()
	}
}

// EnqueueClick enfileira o evento sem bloquear o hot path e sem abrir novas goroutines.
func (p *Publisher) EnqueueClick(event *domain.ClickEvent) {
	select {
	case p.events <- event:
	default:
		// Buffer cheio sob pico extremo: descarta telemetria sem degradar o redirect
	}
}

func (p *Publisher) PublishClick(ctx context.Context, event *domain.ClickEvent) error {
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("erro ao serializar click event: %w", err)
	}

	p.pubMu.Lock()
	defer p.pubMu.Unlock()

	return p.channel.PublishWithContext(
		ctx,
		ExchangeClicks,
		RoutingKey,
		false, // mandatory
		false, // immediate
		amqp.Publishing{
			ContentType:  "application/json",
			DeliveryMode: amqp.Persistent,
			Body:         data,
		},
	)
}

func (p *Publisher) Close() {
	p.closeOnce.Do(func() {
		if p.events != nil {
			close(p.events)
			<-p.done
		}
		if p.channel != nil {
			_ = p.channel.Close()
		}
		if p.conn != nil {
			_ = p.conn.Close()
		}
	})
}

// Consumer gerencia o processamento assíncrono de eventos de clique
type Consumer struct {
	conn    *amqp.Connection
	channel *amqp.Channel
	queue   string
}

func NewConsumer(url string) (*Consumer, error) {
	conn, err := amqp.Dial(url)
	if err != nil {
		return nil, fmt.Errorf("erro ao conectar no rabbitmq consumer: %w", err)
	}

	ch, err := conn.Channel()
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("erro ao abrir canal rabbitmq consumer: %w", err)
	}

	// Garante que a exchange existe
	_ = ch.ExchangeDeclare(ExchangeClicks, "topic", true, false, false, false, nil)

	// Declara fila durável
	q, err := ch.QueueDeclare(
		QueueClicks,
		true,  // durable
		false, // auto-delete
		false, // exclusive
		false, // no-wait
		nil,
	)
	if err != nil {
		ch.Close()
		conn.Close()
		return nil, fmt.Errorf("erro ao declarar fila: %w", err)
	}

	// Faz o bind da fila na exchange
	err = ch.QueueBind(
		q.Name,
		RoutingKey,
		ExchangeClicks,
		false,
		nil,
	)
	if err != nil {
		ch.Close()
		conn.Close()
		return nil, fmt.Errorf("erro ao vincular fila à exchange: %w", err)
	}

	return &Consumer{
		conn:    conn,
		channel: ch,
		queue:   q.Name,
	}, nil
}

func (c *Consumer) Consume(handler func(event *domain.ClickEvent) error) error {
	msgs, err := c.channel.Consume(
		c.queue,
		"",    // consumer tag
		false, // auto-ack (faremos ack manual para confiabilidade)
		false, // exclusive
		false, // no-local
		false, // no-wait
		nil,
	)
	if err != nil {
		return fmt.Errorf("erro ao iniciar consumo: %w", err)
	}

	for d := range msgs {
		var event domain.ClickEvent
		if err := json.Unmarshal(d.Body, &event); err != nil {
			log.Printf("[Worker] Erro ao deserializar mensagem: %v", err)
			_ = d.Nack(false, false) // descarta mensagem corrompida
			continue
		}

		if err := handler(&event); err != nil {
			log.Printf("[Worker] Erro ao processar evento de clique: %v", err)
			_ = d.Nack(false, true) // reenfileira
		} else {
			_ = d.Ack(false)
		}
	}

	return nil
}

func (c *Consumer) Close() {
	if c.channel != nil {
		_ = c.channel.Close()
	}
	if c.conn != nil {
		_ = c.conn.Close()
	}
}
