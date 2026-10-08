package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"golang.org/x/sync/singleflight"
)

const (
	codexLatestReleaseURL        = "https://api.github.com/repos/openai/codex/releases/latest"
	codexClientVersionCacheTTL   = time.Hour
	codexClientVersionFailureTTL = 30 * time.Second
	codexModelsResponseMaxBytes  = 4 << 20
)

type codexClientVersionCache struct {
	sync.Mutex
	fetchGroup singleflight.Group
	version    string
	expiresAt  time.Time
	lastError  error
	retryAt    time.Time
}

var latestCodexClientVersion codexClientVersionCache

func GetLatestCodexClientVersion(ctx context.Context, client *http.Client) (string, error) {
	return latestCodexClientVersion.get(ctx, client, codexLatestReleaseURL, time.Now())
}

func (cache *codexClientVersionCache) get(ctx context.Context, client *http.Client, releaseURL string, now time.Time) (string, error) {
	cache.Lock()
	if cache.version != "" && now.Before(cache.expiresAt) {
		version := cache.version
		cache.Unlock()
		return version, nil
	}
	if cache.version == "" && cache.lastError != nil && now.Before(cache.retryAt) {
		err := cache.lastError
		cache.Unlock()
		return "", err
	}
	cache.Unlock()
	completion := cache.fetchGroup.DoChan(releaseURL, func() (any, error) {
		cache.Lock()
		if cache.version != "" && now.Before(cache.expiresAt) {
			version := cache.version
			cache.Unlock()
			return version, nil
		}
		if cache.version == "" && cache.lastError != nil && now.Before(cache.retryAt) {
			err := cache.lastError
			cache.Unlock()
			return "", err
		}
		cache.Unlock()

		version, err := fetchLatestCodexClientVersion(ctx, client, releaseURL)
		cache.Lock()
		defer cache.Unlock()
		if err != nil {
			if cache.version != "" {
				cache.expiresAt = now.Add(codexClientVersionCacheTTL)
				return cache.version, nil
			}
			cache.lastError = err
			cache.retryAt = now.Add(codexClientVersionFailureTTL)
			return "", err
		}

		cache.version = version
		cache.expiresAt = now.Add(codexClientVersionCacheTTL)
		cache.lastError = nil
		cache.retryAt = time.Time{}
		return version, nil
	})
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	case result := <-completion:
		if result.Err != nil {
			return "", result.Err
		}
		return result.Val.(string), nil
	}
}

func fetchLatestCodexClientVersion(ctx context.Context, client *http.Client, releaseURL string) (string, error) {
	if client == nil {
		return "", fmt.Errorf("nil http client")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, releaseURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "new-api")

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return "", fmt.Errorf("codex release lookup failed: status=%d", resp.StatusCode)
	}

	var release struct {
		TagName    string `json:"tag_name"`
		Name       string `json:"name"`
		Draft      bool   `json:"draft"`
		Prerelease bool   `json:"prerelease"`
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, (1<<20)+1))
	if err != nil || len(body) > 1<<20 {
		return "", errors.New("invalid codex release response")
	}
	if err := common.Unmarshal(body, &release); err != nil {
		return "", err
	}
	version, err := parseStableOfficialCodexReleaseVersion(
		release.TagName,
		release.Name,
		release.Draft,
		release.Prerelease,
	)
	if err != nil {
		return "", err
	}
	return version, nil
}

func FetchCodexModels(
	ctx context.Context,
	client *http.Client,
	baseURL string,
	oauthKey *CodexOAuthKey,
	clientVersion string,
) (statusCode int, models []string, err error) {
	if client == nil {
		return 0, nil, fmt.Errorf("nil http client")
	}
	if oauthKey == nil {
		return 0, nil, fmt.Errorf("nil oauth key")
	}

	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	accessToken := strings.TrimSpace(oauthKey.AccessToken)
	accountID := strings.TrimSpace(oauthKey.AccountID)
	clientVersion = strings.TrimSpace(clientVersion)
	if baseURL == "" {
		return 0, nil, fmt.Errorf("empty baseURL")
	}
	if accessToken == "" {
		return 0, nil, fmt.Errorf("codex channel: access_token is required")
	}
	if accountID == "" {
		return 0, nil, fmt.Errorf("codex channel: account_id is required")
	}
	if clientVersion == "" {
		return 0, nil, fmt.Errorf("codex channel: client_version is required")
	}
	identity := ResolveCodexClientIdentity()
	if IsCodexClientIdentityEnforced() {
		clientVersion = identity.Version
	}

	modelsURL, err := url.Parse(baseURL + "/backend-api/codex/models")
	if err != nil {
		return 0, nil, err
	}
	query := modelsURL.Query()
	query.Set("client_version", clientVersion)
	modelsURL.RawQuery = query.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, modelsURL.String(), nil)
	if err != nil {
		return 0, nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("ChatGPT-Account-Id", accountID)
	req.Header.Set("Accept", "application/json")
	if IsCodexClientIdentityEnforced() {
		ApplyCodexInferenceIdentitySnapshot(req.Header, identity)
	} else {
		req.Header.Set("User-Agent", "codex-cli/"+clientVersion)
	}

	resp, err := client.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return resp.StatusCode, nil, nil
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, codexModelsResponseMaxBytes+1))
	if err != nil {
		return resp.StatusCode, nil, errors.New("codex models response could not be read")
	}
	if len(body) > codexModelsResponseMaxBytes {
		return resp.StatusCode, nil, errors.New("codex models response exceeds size limit")
	}

	var result struct {
		Models []struct {
			Slug string `json:"slug"`
		} `json:"models"`
	}
	if err := common.Unmarshal(body, &result); err != nil {
		return resp.StatusCode, nil, err
	}

	seen := make(map[string]struct{}, len(result.Models))
	models = make([]string, 0, len(result.Models))
	for _, item := range result.Models {
		slug := strings.TrimSpace(item.Slug)
		if slug == "" {
			continue
		}
		if _, ok := seen[slug]; ok {
			continue
		}
		seen[slug] = struct{}{}
		models = append(models, slug)
	}
	return resp.StatusCode, models, nil
}
