package middleware

import (
	"crypto/sha256"
	"crypto/subtle"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"

	"github.com/gin-gonic/gin"
)

const PrometheusMetricsTokenEnv = "PROMETHEUS_METRICS_TOKEN"

func PrometheusMetricsAuth() gin.HandlerFunc {
	authFailureRateLimit := PrometheusMetricsRateLimit()
	return func(c *gin.Context) {
		expected := strings.TrimSpace(common.GetEnvOrDefaultString(PrometheusMetricsTokenEnv, ""))
		if expected == "" {
			c.String(http.StatusServiceUnavailable, "prometheus metrics token not configured")
			c.Abort()
			return
		}
		provided := prometheusMetricsBearer(c.GetHeader("Authorization"))
		if provided == "" || !prometheusMetricsTokenEqual(expected, provided) {
			authFailureRateLimit(c)
			if c.IsAborted() {
				return
			}
			c.Header("WWW-Authenticate", `Bearer realm="prometheus"`)
			c.String(http.StatusUnauthorized, "unauthorized")
			c.Abort()
			return
		}
		c.Next()
	}
}

func prometheusMetricsBearer(header string) string {
	parts := strings.Fields(header)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return ""
	}
	return parts[1]
}

func prometheusMetricsTokenEqual(expected, provided string) bool {
	expectedHash := sha256.Sum256([]byte(expected))
	providedHash := sha256.Sum256([]byte(provided))
	return subtle.ConstantTimeCompare(expectedHash[:], providedHash[:]) == 1
}
