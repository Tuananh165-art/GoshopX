package internal

import (
	"context"
	"fmt"
	"html"
	"mime"
	"net/smtp"
	"strings"
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
	message := []byte(composeEmail(sender.from, recipient, subject, body))
	return smtp.SendMail(sender.host+":"+sender.port, sender.auth, sender.from, []string{recipient}, message)
}

func composeEmail(from, recipient, subject, body string) string {
	const boundary = "goshopx-notification-boundary"
	plainBody := strings.ReplaceAll(body, "\n", "\r\n")
	encodedSubject := mime.QEncoding.Encode("UTF-8", strings.ReplaceAll(strings.ReplaceAll(subject, "\r", ""), "\n", ""))

	return fmt.Sprintf(
		"From: %s\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: multipart/alternative; boundary=%q\r\n\r\n"+
			"--%s\r\nContent-Type: text/plain; charset=UTF-8\r\nContent-Transfer-Encoding: 8bit\r\n\r\n%s\r\n"+
			"--%s\r\nContent-Type: text/html; charset=UTF-8\r\nContent-Transfer-Encoding: 8bit\r\n\r\n%s\r\n"+
			"--%s--\r\n",
		from, recipient, encodedSubject, boundary, boundary, plainBody, boundary, notificationHTML(body), boundary,
	)
}

func notificationHTML(body string) string {
	escapedBody := html.EscapeString(body)
	escapedBody = strings.ReplaceAll(escapedBody, "\n", "<br>")
	return "<!doctype html><html lang=\"vi\"><body style=\"margin:0;background:#f4f6f8;font-family:Arial,sans-serif;color:#18212f\">" +
		"<table role=\"presentation\" width=\"100%\" cellspacing=\"0\" cellpadding=\"0\" style=\"padding:28px 12px;background:#f4f6f8\"><tr><td align=\"center\">" +
		"<table role=\"presentation\" width=\"600\" cellspacing=\"0\" cellpadding=\"0\" style=\"max-width:600px;background:#ffffff;border-radius:12px;overflow:hidden\">" +
		"<tr><td style=\"padding:22px 30px;background:#171923;color:#ffffff;font-size:24px;font-weight:700\">GOSHOP<span style=\"color:#e40035\">X</span></td></tr>" +
		"<tr><td style=\"padding:30px;font-size:16px;line-height:1.65\">" + escapedBody + "</td></tr>" +
		"<tr><td style=\"padding:18px 30px;background:#f8fafc;color:#64748b;font-size:12px\">Đây là email tự động từ GoshopX. Vui lòng không trả lời email này.</td></tr>" +
		"</table></td></tr></table></body></html>"
}
