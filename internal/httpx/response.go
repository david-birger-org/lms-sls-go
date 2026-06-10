package httpx

import (
	"github.com/gin-gonic/gin"
)

func Error(c *gin.Context, status int, message string) {
	c.JSON(status, gin.H{"error": message})
}

func ErrorMessage(err error, fallback string) string {
	if err == nil {
		return fallback
	}
	if msg := err.Error(); msg != "" {
		return msg
	}
	return fallback
}
