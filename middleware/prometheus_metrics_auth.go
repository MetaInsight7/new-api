package middleware

import (
	"crypto/sha256"
	"crypto/subtle"
	"net"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"

	"github.com/gin-gonic/gin"
)

const PrometheusMetricsTokenEnv = "PROMETHEUS_METRICS_TOKEN"

func PrometheusMetricsAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		if prometheusMetricsLocalScrape(c.Request.RemoteAddr) {
			c.Next()
			return
		}
		expected := strings.TrimSpace(common.GetEnvOrDefaultString(PrometheusMetricsTokenEnv, ""))
		provided := prometheusMetricsBearer(c.GetHeader("Authorization"))
		if expected == "" || provided == "" || !prometheusMetricsTokenEqual(expected, provided) {
			c.Header("WWW-Authenticate", `Bearer realm="prometheus"`)
			c.String(http.StatusUnauthorized, "unauthorized")
			c.Abort()
			return
		}
		c.Next()
	}
}

func prometheusMetricsLocalScrape(remoteAddr string) bool {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		host = remoteAddr
	}
	ip := net.ParseIP(strings.Trim(host, "[]"))
	return ip != nil && ip.IsLoopback()
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
