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
	"net"
	"net/smtp"
	"strconv"
	"strings"
	"time"
)

// dialTimeout — таймаут TCP-подключения к SMTP-серверу. net/smtp.SendMail сам по себе не имеет
// таймаута и может зависнуть навечно, если порт блокируется файрволом молча (без TCP RST) — найдено
// на проде 2026-07-22 (VPS-провайдер блокирует исходящий 587/465, соединение висло часами и
// блокировало обработку всех последующих писем в очереди). Явный net.DialTimeout + ручная передача
// соединения в smtp.NewClient вместо smtp.SendMail — чтобы недоступность SMTP давала быструю ошибку.
const dialTimeout = 15 * time.Second

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
	return sendStartTLS(addr, cfg.Host, auth, cfg.From, msg.To, raw)
}

// sendStartTLS — как net/smtp.SendMail, но с явным таймаутом на TCP-подключение (см. dialTimeout).
func sendStartTLS(addr, host string, auth smtp.Auth, from, to string, raw []byte) error {
	conn, err := net.DialTimeout("tcp", addr, dialTimeout)
	if err != nil {
		return fmt.Errorf("email: dial: %w", err)
	}
	c, err := smtp.NewClient(conn, host)
	if err != nil {
		_ = conn.Close()
		return fmt.Errorf("email: smtp client: %w", err)
	}
	defer func() { _ = c.Close() }()
	if ok, _ := c.Extension("STARTTLS"); ok {
		if err := c.StartTLS(&tls.Config{ServerName: host, MinVersion: tls.VersionTLS12}); err != nil {
			return fmt.Errorf("email: starttls: %w", err)
		}
	}
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

// sendImplicitTLS отправляет письмо по каналу с неявным TLS (порт 465).
func sendImplicitTLS(addr, host string, auth smtp.Auth, from, to string, raw []byte) error {
	dialer := &net.Dialer{Timeout: dialTimeout}
	conn, err := tls.DialWithDialer(dialer, "tcp", addr, &tls.Config{ServerName: host, MinVersion: tls.VersionTLS12})
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
