package service

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/signer/v4"
)

const (
	defaultCodexTempMediaPrefix     = "codex-temp-media"
	defaultCodexTempMediaTTL        = 12 * time.Hour
	maxCodexTempMediaTTL            = 7 * 24 * time.Hour
	defaultCodexTempMediaImageBytes = 20 << 20
	maxCodexTempMediaImageBytes     = 256 << 20
	codexTempMediaHTTPTimeout       = 30 * time.Second
	codexTempMediaSigningService    = "s3"
	codexTempMediaDefaultRegion     = "auto"
	codexTempMediaEnabledEnv        = "CODEX_TEMP_MEDIA_ENABLED"
	codexTempMediaEndpointEnv       = "CODEX_TEMP_MEDIA_S3_ENDPOINT"
	codexTempMediaBucketEnv         = "CODEX_TEMP_MEDIA_S3_BUCKET"
	codexTempMediaRegionEnv         = "CODEX_TEMP_MEDIA_S3_REGION"
	codexTempMediaAccessKeyEnv      = "CODEX_TEMP_MEDIA_S3_ACCESS_KEY"
	codexTempMediaSecretKeyEnv      = "CODEX_TEMP_MEDIA_S3_SECRET_KEY"
	codexTempMediaPrefixEnv         = "CODEX_TEMP_MEDIA_S3_PREFIX"
	codexTempMediaTTLSecondsEnv     = "CODEX_TEMP_MEDIA_PRESIGN_TTL_SECONDS"
	codexTempMediaMaxImageBytesEnv  = "CODEX_TEMP_MEDIA_MAX_IMAGE_BYTES"
)

var (
	ErrCodexTempMediaNotConfigured = errors.New("codex temporary media storage is not configured")
	ErrCodexTempMediaInvalidConfig = errors.New("codex temporary media storage configuration is invalid")
	ErrCodexTempMediaUploadFailed  = errors.New("codex temporary media upload failed")
	ErrCodexTempMediaSignFailed    = errors.New("codex temporary media URL signing failed")
	ErrCodexTempMediaTooLarge      = errors.New("codex temporary media image is too large")
	ErrCodexTempMediaUnsupported   = errors.New("codex temporary media image type is unsupported")

	codexTempMediaNow        = time.Now
	putCodexTempMediaObject  = putCodexTempMediaObjectS3
	signCodexTempMediaObject = signCodexTempMediaObjectS3
)

type CodexTempMediaConfig struct {
	Enabled         bool
	Endpoint        string
	Bucket          string
	Region          string
	AccessKey       string
	SecretKey       string
	KeyPrefix       string
	SignedURLTTL    time.Duration
	MaxImageBytes   int64
	validationError error
}

type CodexTempMediaUploadRequest struct {
	UserID      int
	ContentType string
	Body        []byte
}

type CodexTempMediaUploadResult struct {
	SignedURL   string
	ExpiresAt   int64
	ExpiresIn   int64
	ContentType string
	Size        int
}

func CurrentCodexTempMediaConfig() CodexTempMediaConfig {
	config := CodexTempMediaConfig{
		Enabled:       false,
		Endpoint:      strings.TrimSpace(os.Getenv(codexTempMediaEndpointEnv)),
		Bucket:        strings.TrimSpace(os.Getenv(codexTempMediaBucketEnv)),
		Region:        strings.TrimSpace(os.Getenv(codexTempMediaRegionEnv)),
		AccessKey:     strings.TrimSpace(os.Getenv(codexTempMediaAccessKeyEnv)),
		SecretKey:     strings.TrimSpace(os.Getenv(codexTempMediaSecretKeyEnv)),
		KeyPrefix:     strings.TrimSpace(os.Getenv(codexTempMediaPrefixEnv)),
		SignedURLTTL:  defaultCodexTempMediaTTL,
		MaxImageBytes: defaultCodexTempMediaImageBytes,
	}

	if config.Region == "" {
		config.Region = codexTempMediaDefaultRegion
	}
	if config.KeyPrefix == "" {
		config.KeyPrefix = defaultCodexTempMediaPrefix
	}

	if raw := strings.TrimSpace(os.Getenv(codexTempMediaEnabledEnv)); raw != "" {
		enabled, err := strconv.ParseBool(raw)
		if err != nil {
			config.validationError = ErrCodexTempMediaInvalidConfig
		} else {
			config.Enabled = enabled
		}
	}
	if raw := strings.TrimSpace(os.Getenv(codexTempMediaTTLSecondsEnv)); raw != "" {
		seconds, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || seconds <= 0 || seconds > int64(maxCodexTempMediaTTL/time.Second) {
			config.SignedURLTTL = 0
			config.validationError = ErrCodexTempMediaInvalidConfig
		} else {
			config.SignedURLTTL = time.Duration(seconds) * time.Second
		}
	}
	if raw := strings.TrimSpace(os.Getenv(codexTempMediaMaxImageBytesEnv)); raw != "" {
		bytes, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || bytes <= 0 {
			config.MaxImageBytes = 0
			config.validationError = ErrCodexTempMediaInvalidConfig
		} else {
			config.MaxImageBytes = bytes
		}
	}
	return config
}

func ValidateCodexTempMediaConfiguration() error {
	rawEnabled := strings.TrimSpace(os.Getenv(codexTempMediaEnabledEnv))
	if rawEnabled == "" || strings.EqualFold(rawEnabled, "false") {
		return nil
	}
	return CurrentCodexTempMediaConfig().Validate()
}

func (config CodexTempMediaConfig) Validate() error {
	if config.validationError != nil {
		return config.validationError
	}
	if !config.Enabled {
		return ErrCodexTempMediaNotConfigured
	}
	if config.Endpoint == "" || config.Bucket == "" || config.AccessKey == "" || config.SecretKey == "" {
		return fmt.Errorf("%w: endpoint, bucket and credentials are required", ErrCodexTempMediaInvalidConfig)
	}
	endpoint, err := url.Parse(config.Endpoint)
	if err != nil || endpoint.Host == "" || (endpoint.Scheme != "http" && endpoint.Scheme != "https") || endpoint.User != nil || endpoint.RawQuery != "" || endpoint.Fragment != "" {
		return fmt.Errorf("%w: endpoint must be an HTTP(S) URL without credentials or query parameters", ErrCodexTempMediaInvalidConfig)
	}
	if config.Bucket == "." || config.Bucket == ".." || strings.ContainsAny(config.Bucket, "/\\\x00") || len(config.Bucket) > 63 || !isSafeCodexTempMediaPathPart(config.Bucket) {
		return fmt.Errorf("%w: bucket name is invalid", ErrCodexTempMediaInvalidConfig)
	}
	if config.Region == "" || strings.ContainsAny(config.Region, "\r\n") {
		return fmt.Errorf("%w: region is invalid", ErrCodexTempMediaInvalidConfig)
	}
	if _, err := normalizeCodexTempMediaPrefix(config.KeyPrefix); err != nil {
		return fmt.Errorf("%w: prefix is invalid", ErrCodexTempMediaInvalidConfig)
	}
	if config.SignedURLTTL <= 0 || config.SignedURLTTL > maxCodexTempMediaTTL {
		return fmt.Errorf("%w: signed URL TTL is outside the allowed range", ErrCodexTempMediaInvalidConfig)
	}
	if config.MaxImageBytes <= 0 || config.MaxImageBytes > maxCodexTempMediaImageBytes {
		return fmt.Errorf("%w: maximum image size is outside the allowed range", ErrCodexTempMediaInvalidConfig)
	}
	return nil
}

func UploadCodexTempMedia(ctx context.Context, request CodexTempMediaUploadRequest) (*CodexTempMediaUploadResult, error) {
	config := CurrentCodexTempMediaConfig()
	if err := config.Validate(); err != nil {
		return nil, err
	}
	if len(request.Body) == 0 {
		return nil, ErrCodexTempMediaUploadFailed
	}
	if int64(len(request.Body)) > config.MaxImageBytes {
		return nil, ErrCodexTempMediaTooLarge
	}
	contentType, extension := normalizeCodexTempMediaContentType(request.ContentType)
	if contentType == "" {
		return nil, ErrCodexTempMediaUnsupported
	}
	key, err := buildCodexTempMediaObjectKey(config, request.UserID, extension)
	if err != nil {
		return nil, ErrCodexTempMediaUploadFailed
	}
	if err := putCodexTempMediaObject(ctx, config, key, request.Body, contentType); err != nil {
		return nil, ErrCodexTempMediaUploadFailed
	}

	now := codexTempMediaNow().UTC()
	expiresAt := now.Add(config.SignedURLTTL)
	signedURL, err := signCodexTempMediaObject(ctx, config, key, now)
	if err != nil {
		return nil, ErrCodexTempMediaSignFailed
	}
	return &CodexTempMediaUploadResult{
		SignedURL:   signedURL,
		ExpiresAt:   expiresAt.Unix(),
		ExpiresIn:   int64(config.SignedURLTTL / time.Second),
		ContentType: contentType,
		Size:        len(request.Body),
	}, nil
}

func buildCodexTempMediaObjectKey(config CodexTempMediaConfig, userID int, extension string) (string, error) {
	prefix, err := normalizeCodexTempMediaPrefix(config.KeyPrefix)
	if err != nil {
		return "", err
	}
	random, err := common.GenerateRandomCharsKey(32)
	if err != nil {
		return "", err
	}
	if userID < 0 {
		userID = 0
	}
	return path.Join(prefix, strconv.Itoa(userID), codexTempMediaNow().UTC().Format("20060102"), random+extension), nil
}

func normalizeCodexTempMediaPrefix(prefix string) (string, error) {
	prefix = strings.Trim(strings.TrimSpace(prefix), "/")
	if prefix == "" {
		return defaultCodexTempMediaPrefix, nil
	}
	parts := strings.Split(prefix, "/")
	for _, part := range parts {
		if part == "" || part == "." || part == ".." || strings.ContainsAny(part, "\\\x00\r\n") || !isSafeCodexTempMediaPathPart(part) {
			return "", errors.New("invalid object prefix")
		}
	}
	return strings.Join(parts, "/"), nil
}

func isSafeCodexTempMediaPathPart(part string) bool {
	for _, char := range part {
		if char >= 'a' && char <= 'z' || char >= 'A' && char <= 'Z' || char >= '0' && char <= '9' || char == '-' || char == '_' || char == '.' {
			continue
		}
		return false
	}
	return true
}

func normalizeCodexTempMediaContentType(contentType string) (string, string) {
	switch strings.ToLower(strings.TrimSpace(strings.Split(contentType, ";")[0])) {
	case "image/png":
		return "image/png", ".png"
	case "image/jpeg", "image/jpg":
		return "image/jpeg", ".jpg"
	case "image/webp":
		return "image/webp", ".webp"
	default:
		return "", ""
	}
}

func codexTempMediaObjectURL(config CodexTempMediaConfig, key string) (string, error) {
	endpoint, err := url.Parse(config.Endpoint)
	if err != nil {
		return "", err
	}
	endpoint.Path = strings.TrimRight(endpoint.Path, "/") + "/" + config.Bucket + "/" + key
	endpoint.RawPath = ""
	return endpoint.String(), nil
}

func putCodexTempMediaObjectS3(ctx context.Context, config CodexTempMediaConfig, key string, body []byte, contentType string) error {
	objectURL, err := codexTempMediaObjectURL(config, key)
	if err != nil {
		return err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPut, objectURL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", contentType)
	request.Header.Set("Cache-Control", fmt.Sprintf("private, max-age=%d", int64(config.SignedURLTTL/time.Second)))
	hash := sha256.Sum256(body)
	credentials := aws.Credentials{AccessKeyID: config.AccessKey, SecretAccessKey: config.SecretKey}
	if err := v4.NewSigner().SignHTTP(ctx, credentials, request, hex.EncodeToString(hash[:]), codexTempMediaSigningService, config.Region, codexTempMediaNow().UTC()); err != nil {
		return err
	}
	client := &http.Client{Timeout: codexTempMediaHTTPTimeout}
	response, err := client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 4096))
		return fmt.Errorf("object storage returned status %d", response.StatusCode)
	}
	return nil
}

func signCodexTempMediaObjectS3(ctx context.Context, config CodexTempMediaConfig, key string, signingTime time.Time) (string, error) {
	objectURL, err := codexTempMediaObjectURL(config, key)
	if err != nil {
		return "", err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, objectURL, nil)
	if err != nil {
		return "", err
	}
	query := request.URL.Query()
	query.Set("X-Amz-Expires", strconv.FormatInt(int64(config.SignedURLTTL/time.Second), 10))
	request.URL.RawQuery = query.Encode()
	credentials := aws.Credentials{AccessKeyID: config.AccessKey, SecretAccessKey: config.SecretKey}
	signedURL, _, err := v4.NewSigner().PresignHTTP(ctx, credentials, request, "UNSIGNED-PAYLOAD", codexTempMediaSigningService, config.Region, signingTime.UTC())
	if err != nil {
		return "", err
	}
	return signedURL, nil
}
