package service

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type codexVersionSyncRoundTripper func(*http.Request) (*http.Response, error)

func (fn codexVersionSyncRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func setupCodexVersionSyncTest(t *testing.T) {
	t.Helper()
	originalDB := model.DB
	originalOptionMap := common.OptionMap
	originalMaster := common.IsMasterNode
	originalClient := codexVersionSyncHTTPClient
	originalURL := codexVersionSyncReleaseURL

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.Option{}))

	model.DB = db
	common.IsMasterNode = true
	common.OptionMapRWMutex.Lock()
	common.OptionMap = map[string]string{
		OptionKeyCodexAutoSyncClientVersion: "true",
		OptionKeyCodexSyncedClientVersion:   "0.144.0",
		OptionKeyCodexSyncedClientVersionAt: "",
		OptionKeyCodexEnforceClientIdentity: "true",
		OptionKeyCodexClientVersion:         "",
		OptionKeyCodexClientUserAgent:       "",
	}
	common.OptionMapRWMutex.Unlock()
	codexVersionSyncReleaseURL = "https://codex-version-sync.test/releases/latest"

	t.Cleanup(func() {
		model.DB = originalDB
		common.IsMasterNode = originalMaster
		common.OptionMapRWMutex.Lock()
		common.OptionMap = originalOptionMap
		common.OptionMapRWMutex.Unlock()
		codexVersionSyncHTTPClient = originalClient
		codexVersionSyncReleaseURL = originalURL
	})
}

func TestParseStableOfficialCodexReleaseVersion(t *testing.T) {
	version, err := parseStableOfficialCodexReleaseVersion("rust-v0.200.0", "", false, false)
	require.NoError(t, err)
	require.Equal(t, "0.200.0", version)

	_, err = parseStableOfficialCodexReleaseVersion("rust-v0.201.0-rc.1", "", false, false)
	require.Error(t, err)
	_, err = parseStableOfficialCodexReleaseVersion("", "rust-v0.201.0", true, false)
	require.Error(t, err)
}

func TestSyncLatestStableCodexVersionPreservesPreviousValueOnFailure(t *testing.T) {
	setupCodexVersionSyncTest(t)
	require.NoError(t, model.UpdateOption(OptionKeyCodexSyncedClientVersion, "0.199.0"))
	require.NoError(t, model.UpdateOption(OptionKeyCodexSyncedClientVersionAt, "2026-08-20T00:00:00Z"))
	codexVersionSyncHTTPClient = &http.Client{Transport: codexVersionSyncRoundTripper(func(*http.Request) (*http.Response, error) {
		return nil, errors.New("release service unavailable")
	})}

	require.NoError(t, SyncLatestStableCodexVersion(context.Background()))

	var option model.Option
	require.NoError(t, model.DB.Where("key = ?", OptionKeyCodexSyncedClientVersion).First(&option).Error)
	require.Equal(t, "0.199.0", option.Value)
	option = model.Option{}
	require.NoError(t, model.DB.Where("key = ?", OptionKeyCodexSyncedClientVersionAt).First(&option).Error)
	require.Equal(t, "2026-08-20T00:00:00Z", option.Value)
}

func TestSyncLatestStableCodexVersionSkipsNetworkWhenDisabled(t *testing.T) {
	setupCodexVersionSyncTest(t)
	require.NoError(t, model.UpdateOption(OptionKeyCodexAutoSyncClientVersion, "false"))
	called := false
	codexVersionSyncHTTPClient = &http.Client{Transport: codexVersionSyncRoundTripper(func(*http.Request) (*http.Response, error) {
		called = true
		return nil, errors.New("network should not be used")
	})}

	require.NoError(t, SyncLatestStableCodexVersion(context.Background()))
	require.False(t, called)
}

func TestSyncLatestStableCodexVersionPersistsOnlyNewerStableRelease(t *testing.T) {
	setupCodexVersionSyncTest(t)
	require.NoError(t, model.UpdateOption(OptionKeyCodexSyncedClientVersion, "0.199.0"))
	codexVersionSyncHTTPClient = &http.Client{Transport: codexVersionSyncRoundTripper(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(`{"tag_name":"rust-v0.200.0","draft":false,"prerelease":false}`)),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Request:    req,
		}, nil
	})}

	require.NoError(t, SyncLatestStableCodexVersion(context.Background()))

	var option model.Option
	require.NoError(t, model.DB.Where("key = ?", OptionKeyCodexSyncedClientVersion).First(&option).Error)
	require.Equal(t, "0.200.0", option.Value)
	require.NotEmpty(t, requireOptionValue(t, OptionKeyCodexSyncedClientVersionAt))
}

func TestSyncLatestStableCodexVersionDoesNotDowngradePersistedVersion(t *testing.T) {
	setupCodexVersionSyncTest(t)
	require.NoError(t, model.DB.Create(&model.Option{Key: OptionKeyCodexSyncedClientVersion, Value: "0.202.0"}).Error)
	common.OptionMapRWMutex.Lock()
	common.OptionMap[OptionKeyCodexSyncedClientVersion] = "0.199.0"
	common.OptionMapRWMutex.Unlock()
	codexVersionSyncHTTPClient = &http.Client{Transport: codexVersionSyncRoundTripper(func(request *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(`{"tag_name":"rust-v0.200.0","draft":false,"prerelease":false}`)),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Request:    request,
		}, nil
	})}

	require.NoError(t, SyncLatestStableCodexVersion(context.Background()))
	require.Equal(t, "0.202.0", requireOptionValue(t, OptionKeyCodexSyncedClientVersion))
}

func requireOptionValue(t *testing.T, key string) string {
	t.Helper()
	var option model.Option
	require.NoError(t, model.DB.Where("key = ?", key).First(&option).Error)
	return option.Value
}
