package service

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"golang.org/x/sync/singleflight"
)

type CodexCredentialRefreshOptions struct {
	ResetCaches bool
	ExpectedKey string
}

type CodexOAuthKey struct {
	IDToken      string `json:"id_token,omitempty"`
	AccessToken  string `json:"access_token,omitempty"`
	RefreshToken string `json:"refresh_token,omitempty"`

	AccountID   string `json:"account_id,omitempty"`
	LastRefresh string `json:"last_refresh,omitempty"`
	Email       string `json:"email,omitempty"`
	Type        string `json:"type,omitempty"`
	Expired     string `json:"expired,omitempty"`
}

type codexCredentialRefresher struct {
	group singleflight.Group
}

var defaultCodexCredentialRefresher codexCredentialRefresher

// codexCredentialPersistTimeout bounds the detached write so a stuck database
// cannot pin the shared refresh goroutine forever.
const codexCredentialPersistTimeout = 10 * time.Second

type codexCredentialRefreshResult struct {
	key     *CodexOAuthKey
	channel *model.Channel
}

func parseCodexOAuthKey(raw string) (*CodexOAuthKey, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, errors.New("codex channel: empty oauth key")
	}
	var key CodexOAuthKey
	if err := common.Unmarshal([]byte(raw), &key); err != nil {
		return nil, errors.New("codex channel: invalid oauth key json")
	}
	return &key, nil
}

func RefreshCodexChannelCredential(ctx context.Context, channelID int, opts CodexCredentialRefreshOptions) (*CodexOAuthKey, *model.Channel, error) {
	return defaultCodexCredentialRefresher.Refresh(ctx, channelID, opts)
}

func (refresher *codexCredentialRefresher) Refresh(ctx context.Context, channelID int, opts CodexCredentialRefreshOptions) (*CodexOAuthKey, *model.Channel, error) {
	ch, err := model.GetChannelById(channelID, true)
	if err != nil {
		return nil, nil, err
	}
	if ch == nil {
		return nil, nil, fmt.Errorf("channel not found")
	}
	if ch.Type != constant.ChannelTypeCodex {
		return nil, nil, fmt.Errorf("channel type is not Codex")
	}
	if ch.ChannelInfo.IsMultiKey {
		return nil, nil, errors.New("multi-key Codex credential refresh is not supported")
	}

	oauthKey, err := parseCodexOAuthKey(strings.TrimSpace(ch.Key))
	if err != nil {
		return nil, nil, err
	}
	if opts.ExpectedKey != "" && ch.Key != opts.ExpectedKey {
		if opts.ResetCaches {
			model.InitChannelCache()
		}
		return oauthKey, ch, nil
	}
	if strings.TrimSpace(oauthKey.RefreshToken) == "" {
		return nil, nil, fmt.Errorf("codex channel: refresh_token is required to refresh credential")
	}
	operationKey := strconv.Itoa(channelID) + ":" + common.GenerateHMAC(ch.Key)
	completion := refresher.group.DoChan(operationKey, func() (any, error) {
		var result codexCredentialRefreshResult
		lockCtx, lockCancel := context.WithTimeout(context.Background(), codexCredentialPersistTimeout)
		defer lockCancel()
		err := model.WithPluginCredentialRefreshLock(lockCtx, channelID, func() error {
			current, err := model.GetChannelById(channelID, true)
			if err != nil {
				return err
			}
			if current.Type != constant.ChannelTypeCodex || current.ChannelInfo.IsMultiKey {
				return errors.New("Codex channel configuration changed")
			}
			if current.Key != ch.Key {
				key, err := parseCodexOAuthKey(current.Key)
				result = codexCredentialRefreshResult{key: key, channel: current}
				return err
			}

			currentKey, err := parseCodexOAuthKey(current.Key)
			if err != nil {
				return err
			}
			if strings.TrimSpace(currentKey.RefreshToken) == "" {
				return fmt.Errorf("codex channel: refresh_token is required to refresh credential")
			}

			refreshCtx, refreshCancel := context.WithTimeout(context.Background(), codexCredentialPersistTimeout)
			defer refreshCancel()
			res, err := RefreshCodexOAuthTokenWithProxy(refreshCtx, currentKey.RefreshToken, current.GetSetting().Proxy)
			if err != nil {
				return err
			}

			currentKey.AccessToken = res.AccessToken
			currentKey.RefreshToken = res.RefreshToken
			currentKey.LastRefresh = time.Now().Format(time.RFC3339)
			currentKey.Expired = res.ExpiresAt.Format(time.RFC3339)
			if strings.TrimSpace(currentKey.Type) == "" {
				currentKey.Type = "codex"
			}
			if strings.TrimSpace(currentKey.AccountID) == "" {
				if accountID, ok := ExtractCodexAccountIDFromJWT(currentKey.AccessToken); ok {
					currentKey.AccountID = accountID
				}
			}
			if strings.TrimSpace(currentKey.Email) == "" {
				if email, ok := ExtractEmailFromJWT(currentKey.AccessToken); ok {
					currentKey.Email = email
				}
			}

			encoded, err := common.Marshal(currentKey)
			if err != nil {
				return err
			}

			// The upstream has already rotated the refresh token for every waiter in
			// this process and on every node holding the credential lock. No single
			// request cancellation may discard it.
			persistCtx, persistCancel := context.WithTimeout(context.Background(), codexCredentialPersistTimeout)
			defer persistCancel()
			updated := model.DB.WithContext(persistCtx).Model(&model.Channel{}).Where(map[string]any{"id": current.Id, "type": constant.ChannelTypeCodex, "key": current.Key}).Update("key", string(encoded))
			if updated.Error != nil {
				return errors.New("failed to persist refreshed Codex credential")
			}
			if updated.RowsAffected != 1 {
				latest, err := model.GetChannelById(channelID, true)
				if err != nil {
					return err
				}
				if latest.Type != constant.ChannelTypeCodex || latest.ChannelInfo.IsMultiKey {
					return errors.New("Codex channel configuration changed")
				}
				key, err := parseCodexOAuthKey(latest.Key)
				result = codexCredentialRefreshResult{key: key, channel: latest}
				return err
			}
			current.Key = string(encoded)
			model.BumpSyncTopic(model.SyncTopicChannels)
			result = codexCredentialRefreshResult{key: currentKey, channel: current}
			return nil
		})
		if err != nil {
			return nil, err
		}
		return result, nil
	})
	var completed singleflight.Result
	select {
	case <-ctx.Done():
		return nil, nil, ctx.Err()
	case completed = <-completion:
	}
	if completed.Err != nil {
		return nil, nil, completed.Err
	}
	result := completed.Val.(codexCredentialRefreshResult)
	if opts.ResetCaches {
		model.InitChannelCache()
	}
	return result.key, result.channel, nil
}
