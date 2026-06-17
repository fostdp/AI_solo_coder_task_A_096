package metrics

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

func PrometheusMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.FullPath()
		if path == "" {
			path = c.Request.URL.Path
		}
		method := c.Request.Method

		c.Next()

		statusCode := strconv.Itoa(c.Writer.Status())
		duration := time.Since(start)

		HTTPRequestTotal.WithLabelValues(method, path, statusCode).Inc()
		HTTPRequestDuration.WithLabelValues(method, path).Observe(duration.Seconds())
	}
}
