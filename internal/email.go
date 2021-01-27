package internal

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
	"text/template"

	"github.com/go-mail/mail"
	log "github.com/sirupsen/logrus"
)

var (
	ErrSendingEmail = errors.New("error: unable to send email")

	passwordResetEmailTemplate = template.Must(template.New("email").Parse(`Hello {{ .Username }},

You have requested to have your password on {{ .Pod }} reset for your account.

**IMPORTANT:** If this was __NOT__ initiated by you, please ignore this email and contract support!

To reset your password, please visit the following link:

{{ .BaseURL}}/newPassword?token={{ .Token }}

Kind regards,

{{ .Pod}} Support
`))

	supportRequestEmailTemplate = template.Must(template.New("email").Parse(`Hello {{ .AdminUser }},

{{ .Name }} <{{ .Email }} from {{ .Pod }} has sent the following support request:

> Subject: {{ .Subject }}
>
{{ .Message }}

Kind regards,

{{ .Pod}} Support
`))

	reportAbuseEmailTemplate = template.Must(template.New("email").Parse(`Hello {{ .AdminUser }},

{{ .Name }} <{{ .Email }} from {{ .Pod }} has sent the following abuse report:

> Category: {{ .Category }}
>
{{ .Message }}

The offending user/feed in question is:

- Nick: {{ .Nick }}
- URL: {{ .URL }}

Kind regards,

{{ .Pod }} Support
`))
)

type PasswordResetEmailContext struct {
	Pod     string
	BaseURL string

	Token    string
	Username string
}

type SupportRequestEmailContext struct {
	Pod       string
	AdminUser string

	Name    string
	Email   string
	Subject string
	Message string
}

type ReportAbuseEmailContext struct {
	Pod       string
	AdminUser string

	Nick string
	URL  string

	Name     string
	Email    string
	Category string
	Message  string
}

// indents a block of text with an indent string
func Indent(text, indent string) string {
	if text[len(text)-1:] == "\n" {
		result := ""
		for _, j := range strings.Split(text[:len(text)-1], "\n") {
			result += indent + j + "\n"
		}
		return result
	}
	result := ""
	for _, j := range strings.Split(strings.TrimRight(text, "\n"), "\n") {
		result += indent + j + "\n"
	}
	return result[:len(result)-1]
}

func SendEmail(conf *Config, recipients []string, replyTo, subject string, body string) error {
	m := mail.NewMessage()
	m.SetHeader("From", conf.SMTPFrom)
	m.SetHeader("To", recipients...)
	m.SetHeader("Reply-To", replyTo)
	m.SetHeader("Subject", subject)
	m.SetBody("text/plain", body)

	d := mail.NewDialer(conf.SMTPHost, conf.SMTPPort, conf.SMTPUser, conf.SMTPPass)

	err := d.DialAndSend(m)
	if err != nil {
		log.WithError(err).Error("SendEmail() failed")
		return ErrSendingEmail
	}

	return nil
}

func SendPasswordResetEmail(conf *Config, user *User, email, token string) error {
	recipients := []string{email}
	subject := fmt.Sprintf(
		"[%s]: Password Reset Request for %s",
		conf.Name, user.Username,
	)
	ctx := PasswordResetEmailContext{
		Pod:     conf.Name,
		BaseURL: conf.BaseURL,

		Token:    token,
		Username: user.Username,
	}

	buf := &bytes.Buffer{}
	if err := passwordResetEmailTemplate.Execute(buf, ctx); err != nil {
		log.WithError(err).Error("error rendering email template")
		return err
	}

	if err := SendEmail(conf, recipients, conf.SMTPFrom, subject, buf.String()); err != nil {
		log.WithError(err).Errorf("error sending new token to %s", recipients[0])
		return err
	}

	return nil
}

func SendSupportRequestEmail(conf *Config, name, email, subject, message string) error {
	recipients := []string{conf.AdminEmail, email}
	emailSubject := fmt.Sprintf(
		"[%s Support Request]: %s",
		conf.Name, subject,
	)
	ctx := SupportRequestEmailContext{
		Pod:       conf.Name,
		AdminUser: conf.AdminUser,

		Name:    name,
		Email:   email,
		Subject: subject,
		Message: Indent(message, "> "),
	}

	buf := &bytes.Buffer{}
	if err := supportRequestEmailTemplate.Execute(buf, ctx); err != nil {
		log.WithError(err).Error("error rendering email template")
		return err
	}

	if err := SendEmail(conf, recipients, email, emailSubject, buf.String()); err != nil {
		log.WithError(err).Errorf("error sending support request to %s", recipients[0])
		return err
	}

	return nil
}

func SendReportAbuseEmail(conf *Config, nick, url, name, email, category, message string) error {
	recipients := []string{conf.AdminEmail, email}
	emailSubject := fmt.Sprintf(
		"[%s Report Abuse]: %s",
		conf.Name, category,
	)
	ctx := ReportAbuseEmailContext{
		Pod:       conf.Name,
		AdminUser: conf.AdminUser,

		Nick: nick,
		URL:  url,

		Name:     name,
		Email:    email,
		Category: category,
		Message:  Indent(message, "> "),
	}

	buf := &bytes.Buffer{}
	if err := reportAbuseEmailTemplate.Execute(buf, ctx); err != nil {
		log.WithError(err).Error("error rendering email template")
		return err
	}

	if err := SendEmail(conf, recipients, email, emailSubject, buf.String()); err != nil {
		log.WithError(err).Errorf("error sending report abuse to %s", recipients[0])
		return err
	}

	return nil
}
