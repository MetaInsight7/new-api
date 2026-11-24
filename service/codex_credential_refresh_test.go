package service

import (
	"context"
	"io"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type codexCredentialRefreshRoundTripper func(*http.Request) (*http.Response, error)

func (roundTripper codexCredentialRefreshRoundTripper) RoundTrip(request *http.Request) (*http.Response, error) {
	return roundTripper(request)
}

type codexCredentialCancelOnClose struct {
	io.Reader
	cancel context.CancelFunc
}

func (body codexCredentialCancelOnClose) Close() error {
	body.cancel()
	return nil
}

func useCodexCredentialRefreshDatabase(t *testing.T) *gorm.DB {
	t.Helper()
	database, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "refresh.db")), &gorm.Config{Logger: logger.Discard})
	require.NoError(t, err)
	sqlDatabase, err := database.DB()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, sqlDatabase.Close()) })
	require.NoError(t, database.AutoMigrate(&model.Channel{}, &model.SystemTask{}))

	previousDatabase := model.DB
	model.DB = database
	t.Cleanup(func() { model.DB = previousDatabase })
	previousDatabaseType := common.MainDatabaseType()
	common.SetMainDatabaseType(common.DatabaseTypeSQLite)
	t.Cleanup(func() { common.SetMainDatabaseType(previousDatabaseType) })
	return database
}

func TestRefreshCodexChannelCredentialPersistsRotationAfterWaiterCancellation(t *testing.T) {
	database := useCodexCredentialRefreshDatabase(t)
	channel := model.Channel{
		Type:   constant.ChannelTypeCodex,
		Status: common.ChannelStatusEnabled,
		Key:    `{"access_token":"old-access","refresh_token":"old-refresh","account_id":"same-account"}`,
	}
	require.NoError(t, database.Create(&channel).Error)

	client, err := GetHttpClientWithProxy("")
	require.NoError(t, err)
	previousTransport := client.Transport
	t.Cleanup(func() { client.Transport = previousTransport })
	requestContext, cancel := context.WithCancel(context.Background())
	defer cancel()
	client.Transport = codexCredentialRefreshRoundTripper(func(request *http.Request) (*http.Response, error) {
		require.Equal(t, "auth.openai.com", request.URL.Host)
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body: codexCredentialCancelOnClose{
				Reader: strings.NewReader(`{"access_token":"new-access","refresh_token":"new-refresh","expires_in":3600}`),
				cancel: cancel,
			},
		}, nil
	})

	_, _, _ = RefreshCodexChannelCredential(requestContext, channel.Id, CodexCredentialRefreshOptions{})
	operationKey := strconv.Itoa(channel.Id) + ":" + common.GenerateHMAC(channel.Key)
	_, _, _ = defaultCodexCredentialRefresher.group.Do(operationKey, func() (any, error) { return nil, nil })

	var stored model.Channel
	require.NoError(t, database.First(&stored, channel.Id).Error)
	credential, err := parseCodexOAuthKey(stored.Key)
	require.NoError(t, err)
	require.Equal(t, "new-refresh", credential.RefreshToken)
}

func TestCodexCredentialRefreshersUseOneTokenAcrossNodes(t *testing.T) {
	database := useCodexCredentialRefreshDatabase(t)
	channel := model.Channel{
		Type:   constant.ChannelTypeCodex,
		Status: common.ChannelStatusEnabled,
		Key:    `{"access_token":"old-access","refresh_token":"old-refresh","account_id":"same-account"}`,
	}
	require.NoError(t, database.Create(&channel).Error)

	channelQueriesReady := make(chan struct{})
	var channelQueries atomic.Int32
	var channelQueriesReadyOnce sync.Once
	const queryCallback = "test:codex-credential-refresh-channel-queries"
	require.NoError(t, database.Callback().Query().After("gorm:query").Register(queryCallback, func(database *gorm.DB) {
		if database.Statement != nil && database.Statement.Table == "channels" && channelQueries.Add(1) >= 3 {
			channelQueriesReadyOnce.Do(func() { close(channelQueriesReady) })
		}
	}))
	t.Cleanup(func() { require.NoError(t, database.Callback().Query().Remove(queryCallback)) })

	client, err := GetHttpClientWithProxy("")
	require.NoError(t, err)
	previousTransport := client.Transport
	t.Cleanup(func() { client.Transport = previousTransport })
	refreshStarted := make(chan struct{})
	releaseRefresh := make(chan struct{})
	var refreshCalls atomic.Int32
	client.Transport = codexCredentialRefreshRoundTripper(func(request *http.Request) (*http.Response, error) {
		require.Equal(t, "auth.openai.com", request.URL.Host)
		if refreshCalls.Add(1) == 1 {
			close(refreshStarted)
			<-releaseRefresh
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(`{"access_token":"new-access","refresh_token":"new-refresh","expires_in":3600}`)),
		}, nil
	})

	type refreshResult struct {
		key     *CodexOAuthKey
		channel *model.Channel
		err     error
	}
	results := make(chan refreshResult, 2)
	nodes := []*codexCredentialRefresher{{}, {}}
	go func() {
		key, refreshedChannel, refreshErr := nodes[0].Refresh(context.Background(), channel.Id, CodexCredentialRefreshOptions{})
		results <- refreshResult{key: key, channel: refreshedChannel, err: refreshErr}
	}()
	<-refreshStarted
	go func() {
		key, refreshedChannel, refreshErr := nodes[1].Refresh(context.Background(), channel.Id, CodexCredentialRefreshOptions{})
		results <- refreshResult{key: key, channel: refreshedChannel, err: refreshErr}
	}()
	<-channelQueriesReady
	close(releaseRefresh)

	for range nodes {
		result := <-results
		require.NoError(t, result.err)
		require.NotNil(t, result.key)
		require.NotNil(t, result.channel)
		assert.Equal(t, "new-refresh", result.key.RefreshToken)
	}
	assert.EqualValues(t, 1, refreshCalls.Load(), "a refresh token used twice can revoke the whole grant")

	var syncTasks int64
	require.NoError(t, database.Model(&model.SystemTask{}).Where("type = ?", model.SyncTaskType(model.SyncTopicChannels)).Count(&syncTasks).Error)
	assert.EqualValues(t, 1, syncTasks)
}

func TestRefreshCodexChannelCredentialReturnsChangedKeyWithoutRefreshToken(t *testing.T) {
	database := useCodexCredentialRefreshDatabase(t)
	channel := model.Channel{
		Type:   constant.ChannelTypeCodex,
		Status: common.ChannelStatusEnabled,
		Key:    `{"access_token":"current-access","account_id":"same-account"}`,
	}
	require.NoError(t, database.Create(&channel).Error)

	key, refreshedChannel, err := RefreshCodexChannelCredential(context.Background(), channel.Id, CodexCredentialRefreshOptions{ExpectedKey: `{"access_token":"stale"}`})
	require.NoError(t, err)
	require.NotNil(t, refreshedChannel)
	assert.Equal(t, "current-access", key.AccessToken)
	assert.Equal(t, channel.Key, refreshedChannel.Key)
}
