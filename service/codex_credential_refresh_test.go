package service

import (
	"context"
	"io"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/glebarez/sqlite"
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

func TestRefreshCodexChannelCredentialPersistsRotationAfterWaiterCancellation(t *testing.T) {
	database, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "refresh.db")), &gorm.Config{Logger: logger.Discard})
	require.NoError(t, err)
	sqlDatabase, err := database.DB()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, sqlDatabase.Close()) })
	require.NoError(t, database.AutoMigrate(&model.Channel{}))

	previousDatabase := model.DB
	model.DB = database
	t.Cleanup(func() { model.DB = previousDatabase })
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
	_, _, _ = codexCredentialRefreshes.Do(operationKey, func() (any, error) { return nil, nil })

	var stored model.Channel
	require.NoError(t, database.First(&stored, channel.Id).Error)
	credential, err := parseCodexOAuthKey(stored.Key)
	require.NoError(t, err)
	require.Equal(t, "new-refresh", credential.RefreshToken)
}
