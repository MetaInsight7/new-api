package codex

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestSetupRequestHeaderAppliesCanonicalCodexIdentity(t *testing.T) {
	previous := common.OptionMap
	common.OptionMapRWMutex.Lock()
	common.OptionMap = map[string]string{
		"CodexClientVersion":   "0.145.6",
		"CodexClientUserAgent": "codex-cli/0.100.1 relay-test",
	}
	common.OptionMapRWMutex.Unlock()
	t.Cleanup(func() {
		common.OptionMapRWMutex.Lock()
		common.OptionMap = previous
		common.OptionMapRWMutex.Unlock()
	})

	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	info := &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelType: constant.ChannelTypeCodex,
			ApiKey:      `{"access_token":"access","account_id":"account"}`,
			ChannelSetting: dto.ChannelSettings{
				CodexFingerprintMode: dto.CodexFingerprintModeOff,
			},
		},
	}

	headers := http.Header{}
	err := (&Adaptor{}).SetupRequestHeader(c, &headers, info)
	require.NoError(t, err)
	require.Equal(t, "codex-cli/0.145.6 relay-test", headers.Get("User-Agent"))
	require.Equal(t, "codex_cli_rs", headers.Get("originator"))
	require.Equal(t, "0.145.6", headers.Get("OpenAI-Client-Version"))
	require.Equal(t, "Bearer access", headers.Get("Authorization"))
	require.Equal(t, "account", headers.Get("chatgpt-account-id"))
}
