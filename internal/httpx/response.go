package httpx

import (
	"errors"

	"github.com/gin-gonic/gin"
)

func JSON(c *gin.Context, status int, body any) {
	c.JSON(status, body)
}

func Error(c *gin.Context, status int, message string) {
	c.JSON(status, gin.H{"error": message})
}

func ErrorMessage(err error, fallback string) string {
	if err == nil {
		return fallback
	}
	var e interface{ Error() string }
	if errors.As(err, &e) {
		if msg := e.Error(); msg != "" {
			return msg
		}
	}
	return fallback
}
