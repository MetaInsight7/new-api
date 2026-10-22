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

var codexCredentialRefreshes singleflight.Group

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
	if strings.TrimSpace(oauthKey.RefreshToken) == "" {
		return nil, nil, fmt.Errorf("codex channel: refresh_token is required to refresh credential")
	}
	if opts.ExpectedKey != "" && ch.Key != opts.ExpectedKey {
		if opts.ResetCaches {
			model.InitChannelCache()
		}
		return oauthKey, ch, nil
	}
	operationKey := strconv.Itoa(channelID) + ":" + common.GenerateHMAC(ch.Key)
	completion := codexCredentialRefreshes.DoChan(operationKey, func() (any, error) {
		current, err := model.GetChannelById(channelID, true)
		if err != nil {
			return nil, err
		}
		if current.Type != constant.ChannelTypeCodex || current.ChannelInfo.IsMultiKey {
			return nil, errors.New("Codex channel configuration changed")
		}
		if current.Key != ch.Key {
			key, err := parseCodexOAuthKey(current.Key)
			return codexCredentialRefreshResult{key: key, channel: current}, err
		}

		refreshCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()

		res, err := RefreshCodexOAuthTokenWithProxy(refreshCtx, oauthKey.RefreshToken, ch.GetSetting().Proxy)
		if err != nil {
			return nil, err
		}

		oauthKey.AccessToken = res.AccessToken
		oauthKey.RefreshToken = res.RefreshToken
		oauthKey.LastRefresh = time.Now().Format(time.RFC3339)
		oauthKey.Expired = res.ExpiresAt.Format(time.RFC3339)
		if strings.TrimSpace(oauthKey.Type) == "" {
			oauthKey.Type = "codex"
		}

		if strings.TrimSpace(oauthKey.AccountID) == "" {
			if accountID, ok := ExtractCodexAccountIDFromJWT(oauthKey.AccessToken); ok {
				oauthKey.AccountID = accountID
			}
		}
		if strings.TrimSpace(oauthKey.Email) == "" {
			if email, ok := ExtractEmailFromJWT(oauthKey.AccessToken); ok {
				oauthKey.Email = email
			}
		}

		encoded, err := common.Marshal(oauthKey)
		if err != nil {
			return nil, err
		}

		// The upstream has already rotated the refresh token for every waiter in
		// this singleflight call. One request cancellation must not discard it.
		persistCtx, persistCancel := context.WithTimeout(context.WithoutCancel(ctx), codexCredentialPersistTimeout)
		defer persistCancel()
		updated := model.DB.WithContext(persistCtx).Model(&model.Channel{}).Where(map[string]any{"id": ch.Id, "type": constant.ChannelTypeCodex, "key": ch.Key}).Update("key", string(encoded))
		if updated.Error != nil {
			return nil, errors.New("failed to persist refreshed Codex credential")
		}
		if updated.RowsAffected != 1 {
			latest, err := model.GetChannelById(channelID, true)
			if err != nil {
				return nil, err
			}
			if latest.Type != constant.ChannelTypeCodex || latest.ChannelInfo.IsMultiKey {
				return nil, errors.New("Codex channel configuration changed")
			}
			key, err := parseCodexOAuthKey(latest.Key)
			return codexCredentialRefreshResult{key: key, channel: latest}, err
		}
		ch.Key = string(encoded)
		return codexCredentialRefreshResult{key: oauthKey, channel: ch}, nil
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
