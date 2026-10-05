package codex

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/types"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setCodexTempURLTestEnv(t *testing.T, endpoint string) {
	t.Helper()
	for _, key := range []string{
		"CODEX_TEMP_MEDIA_ENABLED",
		"CODEX_TEMP_MEDIA_S3_ENDPOINT",
		"CODEX_TEMP_MEDIA_S3_BUCKET",
		"CODEX_TEMP_MEDIA_S3_REGION",
		"CODEX_TEMP_MEDIA_S3_ACCESS_KEY",
		"CODEX_TEMP_MEDIA_S3_SECRET_KEY",
		"CODEX_TEMP_MEDIA_S3_PREFIX",
		"CODEX_TEMP_MEDIA_PRESIGN_TTL_SECONDS",
		"CODEX_TEMP_MEDIA_MAX_IMAGE_BYTES",
	} {
		t.Setenv(key, "")
	}
	t.Setenv("CODEX_TEMP_MEDIA_ENABLED", "true")
	t.Setenv("CODEX_TEMP_MEDIA_S3_ENDPOINT", endpoint)
	t.Setenv("CODEX_TEMP_MEDIA_S3_BUCKET", "private-images")
	t.Setenv("CODEX_TEMP_MEDIA_S3_REGION", "auto")
	t.Setenv("CODEX_TEMP_MEDIA_S3_ACCESS_KEY", "access-key")
	t.Setenv("CODEX_TEMP_MEDIA_S3_SECRET_KEY", "secret-key")
	t.Setenv("CODEX_TEMP_MEDIA_PRESIGN_TTL_SECONDS", "3600")
}

func TestRelayImageOverCodexReturnsR2TemporaryURL(t *testing.T) {
	var uploaded bool
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		uploaded = true
		assert.Equal(t, http.MethodPut, request.Method)
		assert.Contains(t, request.URL.Path, "/private-images/")
		assert.Contains(t, request.Header.Get("Authorization"), "AWS4-HMAC-SHA256")
		_, _ = io.Copy(io.Discard, request.Body)
		writer.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(server.Close)
	setCodexTempURLTestEnv(t, server.URL)

	tempURL := true
	info := &relaycommon.RelayInfo{
		Request: &dto.ImageRequest{TempUrl: &tempURL},
		UserId:  42,
	}
	sse := strings.Join([]string{
		`data: {"type":"response.output_item.done","item":{"type":"image_generation_call","result":"QUJD"}}`,
		`data: {"type":"response.completed","response":{"created_at":1700000000}}`,
		"",
	}, "\n\n")
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	usage, apiErr := RelayImageOverCodex(c, info, &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(sse)),
	})

	require.Nil(t, apiErr)
	require.NotNil(t, usage)
	assert.True(t, uploaded)
	var response dto.ImageResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	require.Len(t, response.Data, 1)
	assert.Contains(t, response.Data[0].Url, "X-Amz-Signature=")
	assert.Empty(t, response.Data[0].B64Json)
	assert.Equal(t, int64(3600), response.Data[0].ExpiresIn)
	assert.Greater(t, response.Data[0].ExpiresAt, int64(1700000000))
}

func TestRelayImageOverCodexTempURLFailsClosedWhenR2IsUnavailable(t *testing.T) {
	setCodexTempURLTestEnv(t, "")
	t.Setenv("CODEX_TEMP_MEDIA_ENABLED", "false")
	tempURL := true
	info := &relaycommon.RelayInfo{
		Request: &dto.ImageRequest{TempUrl: &tempURL},
		UserId:  42,
	}
	sse := strings.Join([]string{
		`data: {"type":"response.output_item.done","item":{"type":"image_generation_call","result":"QUJD"}}`,
		`data: {"type":"response.completed","response":{"created_at":1700000000}}`,
		"",
	}, "\n\n")
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	usage, apiErr := RelayImageOverCodex(c, info, &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(sse)),
	})

	require.Nil(t, usage)
	require.NotNil(t, apiErr)
	assert.Equal(t, types.ErrorCodeTemporaryMediaUnavailable, apiErr.GetErrorCode())
	assert.Equal(t, http.StatusServiceUnavailable, apiErr.StatusCode)
	assert.Empty(t, recorder.Body.Bytes())
	assert.NotContains(t, apiErr.Error(), "QUJD")
}

func TestCodexTempURLHelpers(t *testing.T) {
	decoded, contentType, err := decodeCodexImageResult("data:image/webp;base64,QUJD")
	require.NoError(t, err)
	assert.Equal(t, []byte("ABC"), decoded)
	assert.Equal(t, "image/webp", contentType)

	outputFormat, err := json.Marshal("jpeg")
	require.NoError(t, err)
	assert.Equal(t, "image/jpeg", codexImageOutputContentType(&dto.ImageRequest{OutputFormat: outputFormat}))

	tempURL := true
	assert.NoError(t, ValidateCodexImageRequest(dto.ImageRequest{TempUrl: &tempURL, ResponseFormat: "url"}))
	assert.Error(t, ValidateCodexImageRequest(dto.ImageRequest{ResponseFormat: "url"}))
}
