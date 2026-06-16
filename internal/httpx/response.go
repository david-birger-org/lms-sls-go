package httpx

import (
	"github.com/gin-gonic/gin"
)

const responseErrorContextKey = "lms.response.error"

func Error(c *gin.Context, status int, message string) {
	c.Set(responseErrorContextKey, message)
	c.JSON(status, gin.H{"error": message})
}

func ResponseError(c *gin.Context) (string, bool) {
	value, ok := c.Get(responseErrorContextKey)
	if !ok {
		return "", false
	}
	message, ok := value.(string)
	return message, ok && message != ""
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
