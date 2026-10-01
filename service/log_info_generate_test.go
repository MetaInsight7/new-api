package service

import (
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestAppendRelayLogAdminInfoRecordsBoundedRequestMetadata(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("POST", "/v1/chat/completions", nil)
	c.Request.Header.Set("User-Agent", "client\n"+string(make([]byte, 600)))
	storage, err := common.CreateBodyStorage([]byte("request-body"))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, storage.Close()) })
	c.Set(common.KeyBodyStorage, storage)

	other := model.NewLogOther()
	AppendRelayLogAdminInfo(c, nil, other)

	adminInfo, ok := other.Snapshot()["admin_info"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, int64(len("request-body")), adminInfo["request_body_size"])
	require.LessOrEqual(t, len(adminInfo["user_agent"].(string)), maxLoggedUserAgentLength)
}
