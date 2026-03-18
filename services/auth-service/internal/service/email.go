package service

import (
	"fmt"
	"net/smtp"
)

// Mailer is the interface for sending transactional emails.
type Mailer interface {
	SendVerification(to, token string) error
	SendPasswordReset(to, token string) error
}

// SMTPMailer sends email via a standard SMTP relay.
type SMTPMailer struct {
	host     string
	port     int
	from     string
	baseURL  string
	username string
	password string
}

// SMTPConfig holds SMTP connection settings.
type SMTPConfig struct {
	Host     string
	Port     int
	From     string
	BaseURL  string
	Username string
	Password string
}

// NewSMTPMailer constructs an SMTPMailer.
func NewSMTPMailer(cfg SMTPConfig) *SMTPMailer {
	return &SMTPMailer{
		host:     cfg.Host,
		port:     cfg.Port,
		from:     cfg.From,
		baseURL:  cfg.BaseURL,
		username: cfg.Username,
		password: cfg.Password,
	}
}

// SendVerification dispatches an email verification link.
func (m *SMTPMailer) SendVerification(to, token string) error {
	subject := "Verify your Relay account"
	link := fmt.Sprintf("%s/auth/verify-email?token=%s", m.baseURL, token)
	body := fmt.Sprintf("Subject: %s\r\nFrom: %s\r\nTo: %s\r\n\r\n"+
		"Click the link below to verify your email:\r\n\r\n%s\r\n\r\nLink expires in 24 hours.\r\n",
		subject, m.from, to, link)
	return m.send(to, []byte(body))
}

// SendPasswordReset dispatches a password-reset link.
func (m *SMTPMailer) SendPasswordReset(to, token string) error {
	subject := "Reset your Relay password"
	link := fmt.Sprintf("%s/auth/reset-password?token=%s", m.baseURL, token)
	body := fmt.Sprintf("Subject: %s\r\nFrom: %s\r\nTo: %s\r\n\r\n"+
		"Click the link below to reset your password:\r\n\r\n%s\r\n\r\nLink expires in 1 hour.\r\n",
		subject, m.from, to, link)
	return m.send(to, []byte(body))
}

func (m *SMTPMailer) send(to string, msg []byte) error {
	addr := fmt.Sprintf("%s:%d", m.host, m.port)
	var auth smtp.Auth
	if m.username != "" {
		auth = smtp.PlainAuth("", m.username, m.password, m.host)
	}
	return smtp.SendMail(addr, auth, m.from, []string{to}, msg)
}

// NoopMailer logs emails instead of sending them (useful for dev/test).
type NoopMailer struct{}

func (n *NoopMailer) SendVerification(to, token string) error {
	fmt.Printf("[noop-mailer] verification email to=%s token=%s\n", to, token)
	return nil
}

func (n *NoopMailer) SendPasswordReset(to, token string) error {
	fmt.Printf("[noop-mailer] password reset email to=%s token=%s\n", to, token)
	return nil
}
