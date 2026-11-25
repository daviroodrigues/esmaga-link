package email

import (
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"net/mail"
	"net/smtp"
	"strings"
	"time"

	"github.com/daviroodrigues/esmaga-api/internal/config"
)

// Sender define a interface agnóstica para envio de e-mails transacionais.
type Sender interface {
	Send(ctx context.Context, to, subject, htmlBody, textBody string) error
}

// NewSender cria um Sender SMTP configurado exclusivamente pelas variáveis de ambiente.
func NewSender(cfg *config.Config) (Sender, error) {
	host := strings.TrimSpace(cfg.SMTPHost)
	if host == "" {
		return nil, errors.New("SMTP_HOST é obrigatório para o envio de e-mails")
	}
	port := strings.TrimSpace(cfg.SMTPPort)
	if port == "" {
		return nil, errors.New("SMTP_PORT é obrigatório para o envio de e-mails")
	}
	from := strings.TrimSpace(cfg.SMTPFrom)
	if from == "" {
		return nil, errors.New("SMTP_FROM é obrigatório para o envio de e-mails")
	}

	return &SMTPSender{
		host:    host,
		port:    port,
		user:    cfg.SMTPUser,
		pass:    cfg.SMTPPass,
		from:    from,
		tlsMode: strings.ToLower(strings.TrimSpace(cfg.SMTPTLS)),
	}, nil
}

// SMTPSender envia e-mails via SMTP padrão (compatível com Mailpit, SES, Resend SMTP, Brevo, etc.).
type SMTPSender struct {
	host    string
	port    string
	user    string
	pass    string
	from    string
	tlsMode string
}

func (s *SMTPSender) Send(ctx context.Context, to, subject, htmlBody, textBody string) error {
	fromAddr, err := mail.ParseAddress(s.from)
	if err != nil {
		fromAddr = &mail.Address{Address: s.from}
	}

	rawMsg := buildMIMEMessage(s.from, to, subject, textBody, htmlBody)
	addr := net.JoinHostPort(s.host, s.port)

	var conn net.Conn
	dialer := &net.Dialer{Timeout: 10 * time.Second}
	if s.tlsMode == "tls" {
		conn, err = tls.DialWithDialer(dialer, "tcp", addr, &tls.Config{ServerName: s.host, MinVersion: tls.VersionTLS12})
	} else {
		conn, err = dialer.DialContext(ctx, "tcp", addr)
	}
	if err != nil {
		return fmt.Errorf("erro ao conectar ao servidor SMTP %s: %w", addr, err)
	}

	client, err := smtp.NewClient(conn, s.host)
	if err != nil {
		conn.Close()
		return fmt.Errorf("erro ao iniciar cliente SMTP: %w", err)
	}
	defer client.Close()

	if s.tlsMode == "starttls" {
		if ok, _ := client.Extension("STARTTLS"); ok {
			if err := client.StartTLS(&tls.Config{ServerName: s.host, MinVersion: tls.VersionTLS12}); err != nil {
				return fmt.Errorf("erro no STARTTLS: %w", err)
			}
		}
	}

	if s.user != "" {
		auth := smtp.PlainAuth("", s.user, s.pass, s.host)
		if err := client.Auth(auth); err != nil {
			return fmt.Errorf("erro na autenticação SMTP: %w", err)
		}
	}

	if err := client.Mail(fromAddr.Address); err != nil {
		return fmt.Errorf("erro no MAIL FROM: %w", err)
	}
	if err := client.Rcpt(to); err != nil {
		return fmt.Errorf("erro no RCPT TO: %w", err)
	}

	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("erro no DATA SMTP: %w", err)
	}
	if _, err := w.Write(rawMsg); err != nil {
		_ = w.Close()
		return fmt.Errorf("erro ao escrever corpo SMTP: %w", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("erro ao finalizar DATA SMTP: %w", err)
	}

	return client.Quit()
}

func buildMIMEMessage(from, to, subject, textBody, htmlBody string) []byte {
	var buf [12]byte
	_, _ = rand.Read(buf[:])
	boundary := "esmaga-" + hex.EncodeToString(buf[:])

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("From: %s\r\n", from))
	sb.WriteString(fmt.Sprintf("To: %s\r\n", to))
	sb.WriteString(fmt.Sprintf("Subject: %s\r\n", subject))
	sb.WriteString("MIME-Version: 1.0\r\n")
	sb.WriteString(fmt.Sprintf("Content-Type: multipart/alternative; boundary=\"%s\"\r\n\r\n", boundary))

	sb.WriteString(fmt.Sprintf("--%s\r\n", boundary))
	sb.WriteString("Content-Type: text/plain; charset=\"UTF-8\"\r\n\r\n")
	sb.WriteString(textBody)
	sb.WriteString("\r\n\r\n")

	sb.WriteString(fmt.Sprintf("--%s\r\n", boundary))
	sb.WriteString("Content-Type: text/html; charset=\"UTF-8\"\r\n\r\n")
	sb.WriteString(htmlBody)
	sb.WriteString("\r\n\r\n")

	sb.WriteString(fmt.Sprintf("--%s--\r\n", boundary))
	return []byte(sb.String())
}
