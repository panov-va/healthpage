// Package email — доставка писем worker-email (этап 3.4): SMTP-отправитель (системный или
// кастомный SMTP страницы, DESIGN §3.5), MIME-сборка multipart/alternative и рендер писем
// по типам событий уведомлений.
package email

import (
	"context"
	"crypto/tls"
	"fmt"
	"log"
	"mime"
	"net/smtp"
	"strconv"
	"strings"
	"time"
)

// SMTP — параметры подключения к SMTP-серверу (системному или из smtp_config страницы).
type SMTP struct {
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Username string `json:"username"`
	Password string `json:"password"`
	From     string `json:"from"` // адрес отправителя
	TLS      bool   `json:"tls"`  // true — неявный TLS (465); иначе STARTTLS
}

// IsZero сообщает, что SMTP не сконфигурирован (нет хоста).
func (c SMTP) IsZero() bool { return c.Host == "" }

// Email — готовое письмо (получатель + контент в двух форматах).
type Email struct {
	To       string
	Subject  string
	TextBody string
	HTMLBody string
}

// Sender отправляет письмо через заданный SMTP. Реализации: SMTPSender (реальная доставка),
// LogSender (dev/без конфигурации — логирует вместо отправки).
type Sender interface {
	Send(ctx context.Context, cfg SMTP, msg Email) error
}

// SMTPSender доставляет письма по SMTP (STARTTLS или неявный TLS).
type SMTPSender struct{}

// Send собирает MIME-сообщение и отправляет его. Аутентификация PLAIN, если задан Username.
func (SMTPSender) Send(_ context.Context, cfg SMTP, msg Email) error {
	if cfg.IsZero() {
		return fmt.Errorf("email: SMTP не сконфигурирован")
	}
	addr := cfg.Host + ":" + strconv.Itoa(cfg.Port)
	raw := buildMIME(cfg.From, msg)

	var auth smtp.Auth
	if cfg.Username != "" {
		auth = smtp.PlainAuth("", cfg.Username, cfg.Password, cfg.Host)
	}

	if cfg.TLS {
		return sendImplicitTLS(addr, cfg.Host, auth, cfg.From, msg.To, raw)
	}
	// STARTTLS-путь: net/smtp.SendMail сам поднимает STARTTLS, если сервер его предлагает.
	return smtp.SendMail(addr, auth, cfg.From, []string{msg.To}, raw)
}

// sendImplicitTLS отправляет письмо по каналу с неявным TLS (порт 465).
func sendImplicitTLS(addr, host string, auth smtp.Auth, from, to string, raw []byte) error {
	conn, err := tls.Dial("tcp", addr, &tls.Config{ServerName: host, MinVersion: tls.VersionTLS12})
	if err != nil {
		return fmt.Errorf("email: tls dial: %w", err)
	}
	c, err := smtp.NewClient(conn, host)
	if err != nil {
		return fmt.Errorf("email: smtp client: %w", err)
	}
	defer func() { _ = c.Close() }()
	if auth != nil {
		if err := c.Auth(auth); err != nil {
			return fmt.Errorf("email: auth: %w", err)
		}
	}
	if err := c.Mail(from); err != nil {
		return fmt.Errorf("email: mail from: %w", err)
	}
	if err := c.Rcpt(to); err != nil {
		return fmt.Errorf("email: rcpt: %w", err)
	}
	wc, err := c.Data()
	if err != nil {
		return fmt.Errorf("email: data: %w", err)
	}
	if _, err := wc.Write(raw); err != nil {
		return fmt.Errorf("email: write body: %w", err)
	}
	if err := wc.Close(); err != nil {
		return fmt.Errorf("email: close body: %w", err)
	}
	return c.Quit()
}

// LogSender ничего не отправляет, только логирует — dev/fallback, когда SMTP не настроен.
type LogSender struct{}

// Send логирует адресата и тему вместо реальной отправки.
func (LogSender) Send(_ context.Context, _ SMTP, msg Email) error {
	log.Printf("email(log): to=%s subject=%q (SMTP не настроен — письмо не отправлено)", msg.To, msg.Subject)
	return nil
}

// buildMIME собирает RFC 5322 / multipart/alternative письмо (text + html).
func buildMIME(from string, msg Email) []byte {
	const boundary = "hp-boundary-7a1b2c3d4e5f"
	var b strings.Builder
	b.WriteString("From: " + from + "\r\n")
	b.WriteString("To: " + msg.To + "\r\n")
	b.WriteString("Subject: " + mime.QEncoding.Encode("utf-8", msg.Subject) + "\r\n")
	b.WriteString("Date: " + time.Now().UTC().Format(time.RFC1123Z) + "\r\n")
	b.WriteString("MIME-Version: 1.0\r\n")
	b.WriteString("Content-Type: multipart/alternative; boundary=\"" + boundary + "\"\r\n\r\n")

	b.WriteString("--" + boundary + "\r\n")
	b.WriteString("Content-Type: text/plain; charset=\"utf-8\"\r\n")
	b.WriteString("Content-Transfer-Encoding: 8bit\r\n\r\n")
	b.WriteString(msg.TextBody + "\r\n\r\n")

	b.WriteString("--" + boundary + "\r\n")
	b.WriteString("Content-Type: text/html; charset=\"utf-8\"\r\n")
	b.WriteString("Content-Transfer-Encoding: 8bit\r\n\r\n")
	b.WriteString(msg.HTMLBody + "\r\n\r\n")

	b.WriteString("--" + boundary + "--\r\n")
	return []byte(b.String())
}
