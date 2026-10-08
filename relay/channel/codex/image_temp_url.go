package codex

import (
	"context"
	"encoding/base64"
	"errors"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
)

func convertCodexImageDataToTempURLs(c *gin.Context, info *relaycommon.RelayInfo, data []dto.ImageData, outputFormats ...string) *types.NewAPIError {
	if !codexImageTempURLRequested(info) {
		return nil
	}
	if info == nil || info.Request == nil {
		return codexTempURLAPIError(service.ErrCodexTempMediaInvalidConfig)
	}
	request, ok := info.Request.(*dto.ImageRequest)
	if !ok {
		return codexTempURLAPIError(service.ErrCodexTempMediaInvalidConfig)
	}

	contentType := codexImageOutputContentType(request)
	ctx := context.Background()
	if c != nil && c.Request != nil {
		ctx = c.Request.Context()
	}
	for index := range data {
		uploadContentType := contentType
		if index < len(outputFormats) && strings.TrimSpace(outputFormats[index]) != "" {
			uploadContentType = codexImageFormatContentType(outputFormats[index])
			if uploadContentType == "" {
				return codexTempURLAPIError(service.ErrCodexTempMediaUnsupported)
			}
		}
		imageBytes, detectedType, err := decodeCodexImageResult(data[index].B64Json)
		if err != nil {
			return types.NewErrorWithStatusCode(errors.New("codex image response contains invalid image data"), types.ErrorCodeBadResponseBody, http.StatusBadGateway, types.ErrOptionWithSkipRetry())
		}
		if detectedType != "" {
			uploadContentType = detectedType
		}
		result, err := service.UploadCodexTempMedia(ctx, service.CodexTempMediaUploadRequest{
			UserID:      info.UserId,
			ContentType: uploadContentType,
			Body:        imageBytes,
		})
		if err != nil {
			return codexTempURLAPIError(err)
		}
		data[index].Url = result.SignedURL
		data[index].B64Json = ""
		data[index].ExpiresAt = result.ExpiresAt
		data[index].ExpiresIn = result.ExpiresIn
	}
	return nil
}

func codexImageTempURLRequested(info *relaycommon.RelayInfo) bool {
	if info == nil || info.Request == nil {
		return false
	}
	request, ok := info.Request.(*dto.ImageRequest)
	return ok && request.TempUrl != nil && *request.TempUrl
}

func decodeCodexImageResult(value string) ([]byte, string, error) {
	value = strings.TrimSpace(value)
	contentType := ""
	if strings.HasPrefix(value, "data:") {
		comma := strings.IndexByte(value, ',')
		if comma < 0 || !strings.Contains(strings.ToLower(value[:comma]), ";base64") {
			return nil, "", errors.New("codex image result is not a base64 data URL")
		}
		contentType = strings.TrimPrefix(value[:comma], "data:")
		if semicolon := strings.IndexByte(contentType, ';'); semicolon >= 0 {
			contentType = contentType[:semicolon]
		}
		value = value[comma+1:]
	}
	if value == "" {
		return nil, contentType, errors.New("codex image result is empty")
	}
	decoded, err := base64.StdEncoding.DecodeString(value)
	if err != nil {
		return nil, contentType, err
	}
	return decoded, contentType, nil
}

func codexImageOutputContentType(request *dto.ImageRequest) string {
	if request == nil || len(request.OutputFormat) == 0 {
		return "image/png"
	}
	var format string
	if err := common.Unmarshal(request.OutputFormat, &format); err != nil {
		return "image/png"
	}
	if contentType := codexImageFormatContentType(format); contentType != "" {
		return contentType
	}
	return "image/png"
}

func codexImageFormatContentType(format string) string {
	switch strings.ToLower(strings.TrimSpace(format)) {
	case "jpg", "jpeg":
		return "image/jpeg"
	case "webp":
		return "image/webp"
	case "png":
		return "image/png"
	default:
		return ""
	}
}

func codexTempURLAPIError(err error) *types.NewAPIError {
	statusCode := http.StatusBadGateway
	message := "codex image temporary URL storage failed"
	switch {
	case errors.Is(err, service.ErrCodexTempMediaNotConfigured):
		statusCode = http.StatusServiceUnavailable
		message = "codex image temporary URLs are not configured"
	case errors.Is(err, service.ErrCodexTempMediaInvalidConfig):
		statusCode = http.StatusServiceUnavailable
		message = "codex image temporary URL storage configuration is invalid"
	case errors.Is(err, service.ErrCodexTempMediaTooLarge):
		message = "codex image is too large for temporary URL storage"
	case errors.Is(err, service.ErrCodexTempMediaUnsupported):
		message = "codex image format is not supported for temporary URL storage"
	}
	return types.NewErrorWithStatusCode(errors.New(message), types.ErrorCodeTemporaryMediaUnavailable, statusCode, types.ErrOptionWithSkipRetry())
}
