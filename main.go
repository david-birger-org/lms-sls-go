package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"

	"github.com/apexwoot/lms-sls-go/internal/auth"
	"github.com/apexwoot/lms-sls-go/internal/db"
	"github.com/apexwoot/lms-sls-go/internal/handlers"
)

func newRouter() *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(gin.LoggerWithWriter(gin.DefaultWriter, "/healthz"))

	r.GET("/healthz", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"ok": true}) })

	api := r.Group("/api")

	api.POST("/contact-requests", handlers.ContactRequestsCreate)

	contactAdmin := api.Group("/contact-requests/admin", auth.RequireAdmin())
	contactAdmin.GET("", handlers.ContactRequestsAdminList)
	contactAdmin.PUT("", handlers.ContactRequestsAdminUpdate)

	api.POST("/internal/app-users/upsert", handlers.InternalAppUsersUpsert)
	api.POST("/mail/transactional", handlers.MailTransactional)

	mbAdmin := api.Group("/monobank", auth.RequireAdmin())
	mbAdmin.POST("/invoice", handlers.MonobankInvoiceCreate)
	mbAdmin.DELETE("/invoice", handlers.MonobankInvoiceDelete)
	mbAdmin.GET("/invoices/pending", handlers.MonobankInvoicesPending)
	mbAdmin.GET("/statement", handlers.MonobankStatement)

	api.POST("/monobank/webhook", handlers.MonobankWebhook)

	paymentsAdmin := api.Group("/payments", auth.RequireAdmin())
	paymentsAdmin.GET("/history", handlers.PaymentsHistory)

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

	return r
}

func main() {
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
