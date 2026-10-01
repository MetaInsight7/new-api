package controller

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/stretchr/testify/assert"
)

func TestSanitizeErrorForUserPreservesActionableRequestErrors(t *testing.T) {
	err := types.NewError(nil, types.ErrorCodeInvalidRequest)
	err.SetMessage("messages cannot be empty (request id: req_upstream_123)")

	sanitizeErrorForUser(err)

	assert.Equal(t, "messages cannot be empty", err.Error())
}

func TestSanitizeErrorForUserHidesUpstreamDetails(t *testing.T) {
	err := types.WithOpenAIError(types.OpenAIError{
		Message: "dial tcp api.example.com:443: connection refused",
		Type:    "server_error",
		Code:    "internal_error",
	}, http.StatusBadGateway)

	sanitizeErrorForUser(err)

	assert.Equal(t, "上游服务异常，请稍后再试", err.Error())
	assert.NotContains(t, err.Error(), "api.example.com")
}

func TestSanitizeErrorForUserHidesInternalChannelDetails(t *testing.T) {
	err := types.NewError(fmt.Errorf("group private-group has no channel for secret-model"), types.ErrorCodeGetChannelFailed)

	sanitizeErrorForUser(err)

	assert.Equal(t, "请求处理失败，请稍后再试", err.Error())
}
