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

func DatabaseURL() (string, error)        { return Required("DATABASE_URL") }
func InternalAPIKey() (string, error)     { return Required("INTERNAL_API_KEY") }
func MonobankToken() (string, error)      { return Required("MONOBANK_TOKEN") }
func MonobankTestToken() (string, error)  { return Required("MONOBANK_TEST_TOKEN") }
func WnbfCheckoutSecret() (string, error) { return Required("WNBF_CHECKOUT_SECRET") }

// PublicAPIKey is an optional, narrowly-scoped key accepted only on the
// public service endpoints (contact requests, transactional mail). When set,
// the marketing site can authenticate with it instead of the admin-capable
// INTERNAL_API_KEY, so a leak there cannot forge admin headers. Empty means
// only INTERNAL_API_KEY is accepted (backward compatible).
func PublicAPIKey() string { return Optional("PUBLIC_API_KEY") }

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
