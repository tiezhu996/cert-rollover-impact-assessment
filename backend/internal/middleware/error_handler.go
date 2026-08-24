package middleware

import (
	"github.com/gin-gonic/gin"
	"log/slog"
)

func ErrorHandler(logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()
		return
	}
}
