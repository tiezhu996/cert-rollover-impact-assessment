package middleware

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"pki-certificate-rollover-impact/backend/internal/util"
)

// Timeout applies a per-request deadline to the request context. Downstream
// handlers, services, repositories, and the database driver all observe the
// same context, so a request that runs past the deadline is cancelled at the
// source rather than left running against a detached context. It must run early
// in the chain so every subsequent middleware shares the deadline.
func Timeout(timeout time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		if timeout <= 0 {
			c.Next()
			return
		}
		ctx, cancel := context.WithTimeout(c.Request.Context(), timeout)
		defer cancel()
		c.Request = c.Request.WithContext(ctx)
		c.Next()

		// If the deadline fired and no response has been written yet, surface an
		// explicit gateway-timeout instead of letting the request hang or return
		// an empty body. When the handler already wrote a response (success or
		// failure), leave it untouched.
		if ctx.Err() != nil && !c.Writer.Written() {
			util.Fail(c, util.NewError(http.StatusGatewayTimeout, util.CodeRequestTimeout, "request exceeded the allowed time budget"))
		}
	}
}
