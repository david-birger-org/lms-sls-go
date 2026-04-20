package mailer

import (
	"fmt"
	"net/smtp"
	"regexp"
	"strings"
	"sync"

	"github.com/apexwoot/lms-sls-go/internal/env"
)

const (
	smtpHost                  = "smtp.gmail.com"
	smtpPort                  = "587"
	defaultDestinationEmail   = "kohut9ra@gmail.com"
)

var emailPattern = regexp.MustCompile(`^[^\s@]+@[^\s@]+\.[^\s@]+$`)

type Input struct {
	Subject string
	Text    string
	ReplyTo string
}

type Result struct {
	OK     bool
	Reason string
	Err    error
}

type Reason = string

const (
	ReasonMissingConfig      Reason = "missing_config"
	ReasonMissingDestination Reason = "missing_destination"
	ReasonSendFailed         Reason = "send_failed"
)

type Mailer struct {
	sender func(from string, to []string, msg []byte) error
}

var (
	defaultMu     sync.Mutex
	defaultSender func(from string, to []string, msg []byte) error
)

func destinationEmail(cfg env.MailConfig) string {
	if cfg.DestinationEmail != "" {
		return cfg.DestinationEmail
	}
	if env.IsProduction() {
		return ""
	}
	return defaultDestinationEmail
}

func defaultSendFn(user, password string) func(from string, to []string, msg []byte) error {
	return func(from string, to []string, msg []byte) error {
		auth := smtp.PlainAuth("", user, password, smtpHost)
		return smtp.SendMail(smtpHost+":"+smtpPort, auth, from, to, msg)
	}
}

func SendTransactional(in Input) Result {
	cfg := env.Mail()
	if cfg.GmailUser == "" || cfg.GmailPassword == "" || cfg.FromAddress == "" {
		return Result{OK: false, Reason: ReasonMissingConfig}
	}
	to := destinationEmail(cfg)
	if to == "" {
		return Result{OK: false, Reason: ReasonMissingDestination}
	}

	defaultMu.Lock()
	sender := defaultSender
	if sender == nil {
		sender = defaultSendFn(cfg.GmailUser, cfg.GmailPassword)
	}
	defaultMu.Unlock()

	msg := buildMessage(cfg.FromAddress, to, in)
	if err := sender(cfg.FromAddress, []string{to}, msg); err != nil {
		return Result{OK: false, Reason: ReasonSendFailed, Err: err}
	}
	return Result{OK: true}
}

func buildMessage(from, to string, in Input) []byte {
	var sb strings.Builder
	fmt.Fprintf(&sb, "From: %s\r\n", from)
	fmt.Fprintf(&sb, "To: %s\r\n", to)
	if in.ReplyTo != "" && emailPattern.MatchString(in.ReplyTo) {
		fmt.Fprintf(&sb, "Reply-To: %s\r\n", in.ReplyTo)
	}
	fmt.Fprintf(&sb, "Subject: %s\r\n", in.Subject)
	sb.WriteString("MIME-Version: 1.0\r\n")
	sb.WriteString("Content-Type: text/plain; charset=UTF-8\r\n")
	sb.WriteString("\r\n")
	sb.WriteString(in.Text)
	return []byte(sb.String())
}
