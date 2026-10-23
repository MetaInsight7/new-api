package controller

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRelayWritesProtocolErrorToCommittedStream(t *testing.T) {
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader("{"))
	ctx.Request.Header.Set("Content-Type", "application/json")
	ctx.Writer.Header().Set("Content-Type", "text/event-stream")
	written := "data: {\"error\":{\"message\":\"stream failed\"}}\n\n"
	_, err := ctx.Writer.WriteString(written)
	require.NoError(t, err)
	Relay(ctx, types.RelayFormatOpenAI)
	body := recorder.Body.String()
	assert.True(t, strings.HasPrefix(body, written))
	assert.Contains(t, body, `data: {"error":`)
	assert.Contains(t, body, `"code":"invalid_request"`)
	assert.True(t, strings.HasSuffix(body, "data: [DONE]\n\n"))
	assert.Equal(t, "text/event-stream", recorder.Header().Get("Content-Type"))
}

func TestRelayKeepsJSONErrorBeforeResponseCommit(t *testing.T) {
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader("{"))
	ctx.Request.Header.Set("Content-Type", "application/json")
	Relay(ctx, types.RelayFormatOpenAI)
	assert.Equal(t, http.StatusBadRequest, recorder.Code)
	assert.Contains(t, recorder.Body.String(), `"error":`)
	assert.Contains(t, recorder.Header().Get("Content-Type"), "application/json")
}
