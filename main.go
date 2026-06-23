package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"runtime/debug"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"

	"github.com/apexwoot/lms-sls-go/internal/auth"
	"github.com/apexwoot/lms-sls-go/internal/db"
	"github.com/apexwoot/lms-sls-go/internal/env"
	"github.com/apexwoot/lms-sls-go/internal/fiscalchecksync"
	"github.com/apexwoot/lms-sls-go/internal/handlers"
	"github.com/apexwoot/lms-sls-go/internal/httpx"
	"github.com/apexwoot/lms-sls-go/internal/monobank"
)

func configureLogger() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})))
}

func requestLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()

		status := c.Writer.Status()
		level := slog.LevelInfo
		if status >= http.StatusInternalServerError {
			level = slog.LevelError
		} else if status >= http.StatusBadRequest {
			level = slog.LevelWarn
		}

		attrs := []any{
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
			"route", c.FullPath(),
			"status", status,
			"latency_ms", float64(time.Since(start).Microseconds()) / 1000,
			"response_bytes", c.Writer.Size(),
			"client_ip", c.ClientIP(),
			"host", c.Request.Host,
			"user_agent", c.Request.UserAgent(),
		}
		if requestID := firstHeader(c.Request, "x-vercel-id", "x-request-id"); requestID != "" {
			attrs = append(attrs, "request_id", requestID)
		}
		if message, ok := httpx.ResponseError(c); ok {
			attrs = append(attrs, "error", message)
		}
		if len(c.Errors) > 0 {
			attrs = append(attrs, "gin_errors", c.Errors.String())
		}

		slog.Log(c.Request.Context(), level, "http request", attrs...)
	}
}

func recoveryLogger() gin.HandlerFunc {
	return gin.CustomRecovery(func(c *gin.Context, recovered any) {
		slog.ErrorContext(c.Request.Context(), "panic recovered",
			"panic", fmt.Sprint(recovered),
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
			"route", c.FullPath(),
			"client_ip", c.ClientIP(),
			"host", c.Request.Host,
			"user_agent", c.Request.UserAgent(),
			"stack", string(debug.Stack()),
		)
		httpx.Error(c, http.StatusInternalServerError, "Internal server error.")
	})
}

func firstHeader(r *http.Request, names ...string) string {
	for _, name := range names {
		if value := r.Header.Get(name); value != "" {
			return value
		}
	}
	return ""
}

func jsonError(status int, message string) gin.HandlerFunc {
	return func(c *gin.Context) {
		httpx.Error(c, status, message)
	}
}

func newRouter() *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.HandleMethodNotAllowed = true
	r.Use(requestLogger())
	r.Use(recoveryLogger())

	r.GET("/healthz", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"ok": true}) })

	api := r.Group("/api")

	api.POST("/contact-requests", auth.RequireServiceKey(), handlers.ContactRequestsCreate)

	contactAdmin := api.Group("/contact-requests/admin", auth.RequireAdmin())
	contactAdmin.GET("", handlers.ContactRequestsAdminList)
	contactAdmin.PUT("", handlers.ContactRequestsAdminUpdate)

	api.POST("/internal/app-users/upsert", auth.RequireInternalKey(), handlers.InternalAppUsersUpsert)
	api.POST("/mail/transactional", auth.RequireServiceKey(), handlers.MailTransactional)
	api.POST("/external/checkout", auth.RequireInternalKey(), handlers.ExternalCheckout)
	api.POST("/external/checkout/test", auth.RequireInternalKey(), handlers.ExternalCheckoutTest)

	mbAdmin := api.Group("/monobank", auth.RequireAdmin())
	mbAdmin.POST("/invoice", handlers.MonobankInvoiceCreate)
	mbAdmin.DELETE("/invoice", handlers.MonobankInvoiceDelete)
	mbAdmin.GET("/invoices/pending", handlers.MonobankInvoicesPending)
	mbAdmin.GET("/statement", handlers.MonobankStatement)

	api.POST("/monobank/webhook", handlers.MonobankWebhook)

	paymentsAdmin := api.Group("/payments", auth.RequireAdmin())
	paymentsAdmin.GET("/history", handlers.PaymentsHistory)

	registrationPaymentsAdmin := api.Group("/registration-payments", auth.RequireAdmin())
	registrationPaymentsAdmin.GET("", handlers.RegistrationPaymentsList)

	api.GET("/products", handlers.ProductsList)

	productsAdmin := api.Group("/products/admin", auth.RequireAdmin())
	productsAdmin.GET("", handlers.ProductsAdminList)
	productsAdmin.POST("", handlers.ProductsAdminCreate)
	productsAdmin.PUT("", handlers.ProductsAdminUpdate)
	productsAdmin.DELETE("", handlers.ProductsAdminDelete)

	user := api.Group("/user", auth.RequireUser())
	user.POST("/checkout", handlers.UserCheckout)
	user.GET("/lectures", handlers.UserLectures)
	user.GET("/purchases", handlers.UserPurchases)

	r.NoRoute(jsonError(http.StatusNotFound, "Not found."))
	r.NoMethod(jsonError(http.StatusMethodNotAllowed, "Method not allowed."))

	return r
}

func shouldStartFiscalCheckWorker() bool {
	return env.Optional("DATABASE_URL") != "" && env.Optional("MONOBANK_TOKEN") != ""
}

func main() {
	configureLogger()

	for _, name := range []string{".env.local", ".env"} {
		if err := godotenv.Load(name); err == nil {
			slog.Info("loaded env file", "file", name)
			break
		}
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "3000"
	}

	server := &http.Server{
		Addr:              ":" + port,
		Handler:           newRouter(),
		ReadHeaderTimeout: 10 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if shouldStartFiscalCheckWorker() {
		fiscalchecksync.StartWorker(ctx, fiscalchecksync.DBStore{}, monobank.NewClient(), fiscalchecksync.WorkerConfig{})
		slog.Info("fiscal check sync worker started")
	} else {
		slog.Info("fiscal check sync worker disabled", "reason", "missing_database_url_or_monobank_token")
	}

	go func() {
		slog.Info("server listening", "port", port)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("server failed", "error", err.Error())
			stop()
		}
	}()

	<-ctx.Done()
	slog.Info("shutting down")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	_ = server.Shutdown(shutdownCtx)
	db.Close()
}
