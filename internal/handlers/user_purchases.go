package handlers

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/apexwoot/lms-sls-go/internal/auth"
	"github.com/apexwoot/lms-sls-go/internal/httpx"
	"github.com/apexwoot/lms-sls-go/internal/userfeatures"
	"github.com/apexwoot/lms-sls-go/internal/userpurchases"
)

const (
	defaultPurchasesLimit     = 100
	maxPurchasesLimit         = 100
	defaultPurchasesRangeDays = 180
)

func purchasesLimit(q string) int {
	if q == "" {
		return defaultPurchasesLimit
	}
	n, err := strconv.Atoi(q)
	if err != nil || n < 1 {
		return defaultPurchasesLimit
	}
	return min(n, maxPurchasesLimit)
}

func normalizeIsoDate(q string, fallback time.Time) time.Time {
	if q == "" {
		return fallback
	}
	t, err := time.Parse(time.RFC3339, q)
	if err != nil {
		return fallback
	}
	return t.UTC()
}

func UserPurchases(c *gin.Context) {
	user := auth.UserFrom(c)
	ctx := c.Request.Context()

	q := c.Request.URL.Query()
	scope := q.Get("scope")
	limit := purchasesLimit(strings.TrimSpace(q.Get("limit")))
	defaultTo := time.Now().UTC()
	defaultFrom := defaultTo.AddDate(0, 0, -defaultPurchasesRangeDays)
	to := normalizeIsoDate(strings.TrimSpace(q.Get("to")), defaultTo)
	from := normalizeIsoDate(strings.TrimSpace(q.Get("from")), defaultFrom)

	query := userpurchases.Query{From: from, Limit: limit, To: to}

	var rows []userpurchases.Row
	var err error
	if scope == "created" {
		rows, err = userpurchases.SelectInvoicesCreatedByAdmin(ctx, user.UserID, query)
	} else {
		rows, err = userpurchases.SelectUserPurchases(ctx, user.UserID, query)
	}
	if err != nil {
		httpx.Error(c, http.StatusInternalServerError, "Failed to fetch purchases: "+err.Error())
		return
	}

	features, err := userfeatures.SelectActiveFeatures(ctx, user.UserID)
	if err != nil {
		httpx.Error(c, http.StatusInternalServerError, "Failed to fetch purchases: "+err.Error())
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"features":  features,
		"purchases": rows,
		"range":     gin.H{"from": from, "to": to},
	})
}
