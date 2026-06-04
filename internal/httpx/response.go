package httpx

import (
	"errors"

	"github.com/gin-gonic/gin"
)

func Error(c *gin.Context, status int, message string) {
	c.JSON(status, gin.H{"error": message})
}

func ErrorMessage(err error, fallback string) string {
	if err == nil {
		return fallback
	}
	if e, ok := errors.AsType[interface{ Error() string }](err); ok {
		if msg := e.Error(); msg != "" {
			return msg
		}
	}
	return fallback
}
