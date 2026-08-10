package internal

import (
	"context"
	"fmt"
	"net/smtp"
)

type EmailSender interface {
	Send(ctx context.Context, recipient, subject, body string) error
}

type smtpEmailSender struct {
	host string
	port string
	from string
	auth smtp.Auth
}

func NewSMTPEmailSender(host, port, username, password, from string) EmailSender {
	return &smtpEmailSender{host: host, port: port, from: from, auth: smtp.PlainAuth("", username, password, host)}
}

func (sender *smtpEmailSender) Send(ctx context.Context, recipient, subject, body string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	message := []byte(fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n%s\r\n", sender.from, recipient, subject, body))
	return smtp.SendMail(sender.host+":"+sender.port, sender.auth, sender.from, []string{recipient}, message)
}
