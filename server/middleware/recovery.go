package middleware

import (
	"net/http"

	"github.com/ReyRen/gcs-distill/internal/logger"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// Recovery 恢复中间件，捕获 panic
func Recovery() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				logger.Error("服务器内部错误",
					zap.Any("error", err),
					zap.String("path", c.Request.URL.Path),
					zap.String("method", c.Request.Method),
				)

				c.JSON(http.StatusInternalServerError, gin.H{
					"code":    http.StatusInternalServerError,
					"message": "服务器内部错误",
				})
				c.Abort()
			}
		}()

		c.Next()
	}
}
