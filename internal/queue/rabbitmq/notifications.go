package rabbitmq

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"esmaga-link/internal/domain"
	amqp "github.com/rabbitmq/amqp091-go"
)

const (
	ExchangeNotifications = "notifications.topic"
	ExchangeDLX           = "notifications.dlx"

	QueueEmail    = "notifications.email"
	QueueEmailDLQ = "notifications.email.dlq"

	RoutingKeyEmailVerification = "email.verification"
	RoutingKeyPasswordReset     = "email.password_reset"
	RoutingKeyAccountDeleted    = "email.account_deleted"
	RoutingKeyEmailRetry        = "email.retry"
	RoutingKeyEmailDLQ          = "email.dlq"
)

type emailRetryTier struct {
	queueName  string
	routingKey string
	delay      time.Duration
}

var emailRetryTiers = []emailRetryTier{
	{queueName: "notifications.email.retry.10s", routingKey: "email.retry_10s", delay: 10 * time.Second},
	{queueName: "notifications.email.retry.1m", routingKey: "email.retry_1m", delay: 1 * time.Minute},
	{queueName: "notifications.email.retry.5m", routingKey: "email.retry_5m", delay: 5 * time.Minute},
}

func declareNotificationsTopology(ch *amqp.Channel) error {
	if err := ch.ExchangeDeclare(ExchangeNotifications, "topic", true, false, false, false, nil); err != nil {
		return fmt.Errorf("erro ao declarar exchange %s: %w", ExchangeNotifications, err)
	}
	if err := ch.ExchangeDeclare(ExchangeDLX, "topic", true, false, false, false, nil); err != nil {
		return fmt.Errorf("erro ao declarar exchange %s: %w", ExchangeDLX, err)
	}

	if _, err := ch.QueueDeclare(QueueEmail, true, false, false, false, amqp.Table{
		"x-dead-letter-exchange":    ExchangeDLX,
		"x-dead-letter-routing-key": RoutingKeyEmailDLQ,
	}); err != nil {
		return fmt.Errorf("erro ao declarar fila %s: %w", QueueEmail, err)
	}
	if err := ch.QueueBind(QueueEmail, "email.*", ExchangeNotifications, false, nil); err != nil {
		return fmt.Errorf("erro ao vincular fila %s: %w", QueueEmail, err)
	}

	for _, tier := range emailRetryTiers {
		if _, err := ch.QueueDeclare(tier.queueName, true, false, false, false, amqp.Table{
			"x-message-ttl":             int64(tier.delay.Milliseconds()),
			"x-dead-letter-exchange":    ExchangeNotifications,
			"x-dead-letter-routing-key": RoutingKeyEmailRetry,
		}); err != nil {
			return fmt.Errorf("erro ao declarar fila %s: %w", tier.queueName, err)
		}
		if err := ch.QueueBind(tier.queueName, tier.routingKey, ExchangeDLX, false, nil); err != nil {
			return fmt.Errorf("erro ao vincular fila %s: %w", tier.queueName, err)
		}
	}

	if _, err := ch.QueueDeclare(QueueEmailDLQ, true, false, false, false, nil); err != nil {
		return fmt.Errorf("erro ao declarar fila %s: %w", QueueEmailDLQ, err)
	}
	if err := ch.QueueBind(QueueEmailDLQ, RoutingKeyEmailDLQ, ExchangeDLX, false, nil); err != nil {
		return fmt.Errorf("erro ao vincular fila %s: %w", QueueEmailDLQ, err)
	}

	return nil
}

func (p *Publisher) PublishEmail(ctx context.Context, routingKey string, msg *domain.EmailMessage) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("erro ao serializar email message: %w", err)
	}

	p.pubMu.Lock()
	defer p.pubMu.Unlock()

	return p.channel.PublishWithContext(
		ctx,
		ExchangeNotifications,
		routingKey,
		false,
		false,
		amqp.Publishing{
			ContentType:  "application/json",
			DeliveryMode: amqp.Persistent,
			MessageId:    msg.IdempotencyKey,
			Body:         data,
		},
	)
}

type EmailConsumer struct {
	conn    *amqp.Connection
	channel *amqp.Channel
}

func NewEmailConsumer(url string) (*EmailConsumer, error) {
	conn, err := amqp.Dial(url)
	if err != nil {
		return nil, fmt.Errorf("erro ao conectar no rabbitmq email consumer: %w", err)
	}

	ch, err := conn.Channel()
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("erro ao abrir canal rabbitmq email consumer: %w", err)
	}

	if err := declareNotificationsTopology(ch); err != nil {
		ch.Close()
		conn.Close()
		return nil, err
	}

	return &EmailConsumer{conn: conn, channel: ch}, nil
}

func (c *EmailConsumer) Consume(handler func(msg *domain.EmailMessage) error) error {
	msgs, err := c.channel.Consume(
		QueueEmail,
		"",
		false,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		return fmt.Errorf("erro ao iniciar consumo de e-mails: %w", err)
	}

	for d := range msgs {
		var msg domain.EmailMessage
		if err := json.Unmarshal(d.Body, &msg); err != nil {
			log.Printf("[Email Worker] Payload inválido, enviando para DLQ: %v", err)
			_ = d.Nack(false, false)
			continue
		}

		if err := handler(&msg); err != nil {
			c.handleFailure(d, err)
			continue
		}

		_ = d.Ack(false)
	}

	return nil
}

func (c *EmailConsumer) handleFailure(d amqp.Delivery, cause error) {
	retries := extractRetryCount(d.Headers)
	if retries >= len(emailRetryTiers) {
		log.Printf("[Email Worker] Limite de tentativas atingido (%d), movendo para DLQ: %v", retries, cause)
		_ = d.Nack(false, false)
		return
	}

	tier := emailRetryTiers[retries]
	headers := amqp.Table{}
	for k, v := range d.Headers {
		headers[k] = v
	}
	headers["x-retry-count"] = int32(retries + 1)
	headers["x-original-routing-key"] = d.RoutingKey

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := c.channel.PublishWithContext(
		ctx,
		ExchangeDLX,
		tier.routingKey,
		false,
		false,
		amqp.Publishing{
			ContentType:  d.ContentType,
			DeliveryMode: amqp.Persistent,
			MessageId:    d.MessageId,
			Headers:      headers,
			Body:         d.Body,
		},
	)
	if err != nil {
		log.Printf("[Email Worker] Falha ao agendar retry, recolocando na fila: %v", err)
		_ = d.Nack(false, true)
		return
	}

	log.Printf("[Email Worker] Erro ao enviar e-mail (tentativa %d/%d), novo envio em %s: %v",
		retries+1, len(emailRetryTiers), tier.delay, cause)
	_ = d.Ack(false)
}

func extractRetryCount(headers amqp.Table) int {
	if headers == nil {
		return 0
	}
	raw, ok := headers["x-retry-count"]
	if !ok {
		return 0
	}
	switch v := raw.(type) {
	case int:
		return v
	case int32:
		return int(v)
	case int64:
		return int(v)
	default:
		return 0
	}
}

func (c *EmailConsumer) Close() {
	if c.channel != nil {
		_ = c.channel.Close()
	}
	if c.conn != nil {
		_ = c.conn.Close()
	}
}
