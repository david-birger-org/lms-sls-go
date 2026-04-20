package env

import (
	"fmt"
	"os"
	"strings"
)

func read(name string) string {
	return strings.TrimSpace(os.Getenv(name))
}

func Optional(name string) string {
	return read(name)
}

func Required(name string) (string, error) {
	v := read(name)
	if v == "" {
		return "", fmt.Errorf("%s is missing in environment variables.", name)
	}
	return v, nil
}

func MustRequired(name string) string {
	v, err := Required(name)
	if err != nil {
		panic(err)
	}
	return v
}

func DatabaseURL() (string, error) { return Required("DATABASE_URL") }
func InternalAPIKey() (string, error) { return Required("INTERNAL_API_KEY") }
func MonobankToken() (string, error)  { return Required("MONOBANK_TOKEN") }

type MailConfig struct {
	GmailUser        string
	GmailPassword    string
	FromAddress      string
	DestinationEmail string
}

func Mail() MailConfig {
	user := read("GMAIL_USER")
	pass := read("GMAIL_PASSWORD")
	if pass == "" {
		pass = read("GMAIL_APP_PASSWORD")
	}
	from := read("SMTP_FROM")
	if from == "" {
		from = user
	}
	return MailConfig{
		GmailUser:        user,
		GmailPassword:    pass,
		FromAddress:      from,
		DestinationEmail: read("MAIL_SEND_TO"),
	}
}

func IsProduction() bool {
	return read("NODE_ENV") == "production" || read("VERCEL_ENV") == "production"
}
