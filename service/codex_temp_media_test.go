package service

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func configureCodexTempMediaTestEnv(t *testing.T) {
	t.Helper()
	for _, key := range []string{
		codexTempMediaEnabledEnv,
		codexTempMediaEndpointEnv,
		codexTempMediaBucketEnv,
		codexTempMediaRegionEnv,
		codexTempMediaAccessKeyEnv,
		codexTempMediaSecretKeyEnv,
		codexTempMediaPrefixEnv,
		codexTempMediaTTLSecondsEnv,
		codexTempMediaMaxImageBytesEnv,
	} {
		t.Setenv(key, "")
	}
	t.Setenv(codexTempMediaEnabledEnv, "true")
	t.Setenv(codexTempMediaEndpointEnv, "https://account.r2.cloudflarestorage.com")
	t.Setenv(codexTempMediaBucketEnv, "private-images")
	t.Setenv(codexTempMediaRegionEnv, "auto")
	t.Setenv(codexTempMediaAccessKeyEnv, "access-key")
	t.Setenv(codexTempMediaSecretKeyEnv, "secret-key")
}

func TestCurrentCodexTempMediaConfigDefaultsAndValidation(t *testing.T) {
	configureCodexTempMediaTestEnv(t)

	config := CurrentCodexTempMediaConfig()
	require.NoError(t, config.Validate())
	assert.Equal(t, defaultCodexTempMediaPrefix, config.KeyPrefix)
	assert.Equal(t, defaultCodexTempMediaTTL, config.SignedURLTTL)
	assert.Equal(t, int64(defaultCodexTempMediaImageBytes), config.MaxImageBytes)
}

func TestCurrentCodexTempMediaConfigRejectsInvalidConfiguration(t *testing.T) {
	configureCodexTempMediaTestEnv(t)
	t.Setenv(codexTempMediaEndpointEnv, "https://account.r2.example?secret=should-not-be-accepted")
	require.ErrorIs(t, CurrentCodexTempMediaConfig().Validate(), ErrCodexTempMediaInvalidConfig)

	t.Setenv(codexTempMediaEndpointEnv, "")
	t.Setenv(codexTempMediaEnabledEnv, "false")
	require.ErrorIs(t, CurrentCodexTempMediaConfig().Validate(), ErrCodexTempMediaNotConfigured)

	t.Setenv(codexTempMediaEnabledEnv, "invalid")
	require.ErrorIs(t, CurrentCodexTempMediaConfig().Validate(), ErrCodexTempMediaInvalidConfig)
}

func TestCurrentCodexTempMediaConfigRejectsTTLAndSizeOutsideLimits(t *testing.T) {
	configureCodexTempMediaTestEnv(t)
	t.Setenv(codexTempMediaTTLSecondsEnv, "604801")
	require.ErrorIs(t, CurrentCodexTempMediaConfig().Validate(), ErrCodexTempMediaInvalidConfig)

	t.Setenv(codexTempMediaTTLSecondsEnv, "43200")
	t.Setenv(codexTempMediaMaxImageBytesEnv, "0")
	require.ErrorIs(t, CurrentCodexTempMediaConfig().Validate(), ErrCodexTempMediaInvalidConfig)
}

func TestValidateCodexTempMediaConfigurationSkipsDisabledStorage(t *testing.T) {
	configureCodexTempMediaTestEnv(t)
	t.Setenv(codexTempMediaEnabledEnv, "false")
	assert.NoError(t, ValidateCodexTempMediaConfiguration())

	t.Setenv(codexTempMediaEnabledEnv, "true")
	t.Setenv(codexTempMediaEndpointEnv, "")
	assert.ErrorIs(t, ValidateCodexTempMediaConfiguration(), ErrCodexTempMediaInvalidConfig)
}

func TestUploadCodexTempMediaUsesPrivateRandomObjectAndSignedURL(t *testing.T) {
	configureCodexTempMediaTestEnv(t)
	t.Setenv(codexTempMediaPrefixEnv, "private/codex")
	t.Setenv(codexTempMediaTTLSecondsEnv, "3600")
	codexTempMediaNow = func() time.Time { return time.Unix(1_700_000_000, 0).UTC() }
	t.Cleanup(func() { codexTempMediaNow = time.Now })

	var uploaded struct {
		key         string
		contentType string
		body        []byte
	}
	oldPut := putCodexTempMediaObject
	oldSign := signCodexTempMediaObject
	putCodexTempMediaObject = func(_ context.Context, _ CodexTempMediaConfig, key string, body []byte, contentType string) error {
		uploaded.key = key
		uploaded.body = append([]byte(nil), body...)
		uploaded.contentType = contentType
		return nil
	}
	signCodexTempMediaObject = func(_ context.Context, _ CodexTempMediaConfig, key string, signingTime time.Time) (string, error) {
		assert.Equal(t, uploaded.key, key)
		assert.Equal(t, time.Unix(1_700_000_000, 0).UTC(), signingTime)
		return "https://signed.example/object", nil
	}
	t.Cleanup(func() {
		putCodexTempMediaObject = oldPut
		signCodexTempMediaObject = oldSign
	})

	result, err := UploadCodexTempMedia(context.Background(), CodexTempMediaUploadRequest{
		UserID:      42,
		ContentType: "image/png; charset=binary",
		Body:        []byte("image-bytes"),
	})

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "https://signed.example/object", result.SignedURL)
	assert.Equal(t, int64(1_700_003_600), result.ExpiresAt)
	assert.Equal(t, int64(3600), result.ExpiresIn)
	assert.Equal(t, "image/png", result.ContentType)
	assert.Equal(t, 11, result.Size)
	assert.Equal(t, []byte("image-bytes"), uploaded.body)
	assert.Equal(t, "image/png", uploaded.contentType)
	assert.Contains(t, uploaded.key, "private/codex/42/20231114/")
	assert.NotContains(t, uploaded.key, "image-bytes")
}

func TestUploadCodexTempMediaRejectsOversizedAndUnsupportedImages(t *testing.T) {
	configureCodexTempMediaTestEnv(t)
	t.Setenv(codexTempMediaMaxImageBytesEnv, "4")

	_, err := UploadCodexTempMedia(context.Background(), CodexTempMediaUploadRequest{
		ContentType: "image/png",
		Body:        []byte("12345"),
	})
	require.ErrorIs(t, err, ErrCodexTempMediaTooLarge)

	t.Setenv(codexTempMediaMaxImageBytesEnv, "20")
	_, err = UploadCodexTempMedia(context.Background(), CodexTempMediaUploadRequest{
		ContentType: "text/plain",
		Body:        []byte("text"),
	})
	require.ErrorIs(t, err, ErrCodexTempMediaUnsupported)
}

func TestUploadCodexTempMediaDoesNotExposeObjectKeyOnStorageFailure(t *testing.T) {
	configureCodexTempMediaTestEnv(t)
	oldPut := putCodexTempMediaObject
	putCodexTempMediaObject = func(_ context.Context, _ CodexTempMediaConfig, _ string, _ []byte, _ string) error {
		return errors.New("upstream storage failure")
	}
	t.Cleanup(func() { putCodexTempMediaObject = oldPut })

	_, err := UploadCodexTempMedia(context.Background(), CodexTempMediaUploadRequest{
		UserID:      42,
		ContentType: "image/png",
		Body:        []byte("image"),
	})

	require.ErrorIs(t, err, ErrCodexTempMediaUploadFailed)
	assert.NotContains(t, err.Error(), "42")
}

func TestUploadCodexTempMediaDoesNotExposeObjectKeyOnSigningFailure(t *testing.T) {
	configureCodexTempMediaTestEnv(t)
	oldPut := putCodexTempMediaObject
	oldSign := signCodexTempMediaObject
	putCodexTempMediaObject = func(_ context.Context, _ CodexTempMediaConfig, _ string, _ []byte, _ string) error {
		return nil
	}
	signCodexTempMediaObject = func(_ context.Context, _ CodexTempMediaConfig, _ string, _ time.Time) (string, error) {
		return "", errors.New("signing failure")
	}
	t.Cleanup(func() {
		putCodexTempMediaObject = oldPut
		signCodexTempMediaObject = oldSign
	})

	_, err := UploadCodexTempMedia(context.Background(), CodexTempMediaUploadRequest{
		UserID:      42,
		ContentType: "image/png",
		Body:        []byte("image"),
	})

	require.ErrorIs(t, err, ErrCodexTempMediaSignFailed)
	assert.NotContains(t, err.Error(), "42")
}

func TestPutCodexTempMediaObjectS3SignsAndSendsPrivateObject(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		assert.Equal(t, "PUT", request.Method)
		assert.Contains(t, request.URL.Path, "/private-images/codex/42.png")
		assert.Equal(t, "image/png", request.Header.Get("Content-Type"))
		assert.Contains(t, request.Header.Get("Authorization"), "AWS4-HMAC-SHA256")
		body, err := io.ReadAll(request.Body)
		require.NoError(t, err)
		assert.Equal(t, "image", string(body))
		writer.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(server.Close)

	config := CodexTempMediaConfig{
		Enabled:       true,
		Endpoint:      server.URL,
		Bucket:        "private-images",
		Region:        "auto",
		AccessKey:     "access-key",
		SecretKey:     "secret-key",
		KeyPrefix:     "codex",
		SignedURLTTL:  time.Hour,
		MaxImageBytes: 20,
	}
	require.NoError(t, config.Validate())
	oldNow := codexTempMediaNow
	codexTempMediaNow = func() time.Time { return time.Unix(1_700_000_000, 0).UTC() }
	t.Cleanup(func() { codexTempMediaNow = oldNow })

	require.NoError(t, putCodexTempMediaObjectS3(context.Background(), config, "codex/42.png", []byte("image"), "image/png"))
}

func TestNormalizeCodexTempMediaPrefixRejectsTraversal(t *testing.T) {
	_, err := normalizeCodexTempMediaPrefix("codex/../private")
	assert.Error(t, err)
	assert.True(t, strings.Contains(err.Error(), "prefix"))
}
