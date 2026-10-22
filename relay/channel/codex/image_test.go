package codex

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/pkg/billingexpr"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/setting/config"
	"github.com/QuantumNous/new-api/setting/model_setting"
	hosttypes "github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type codexImageReadAfterCompleted struct{}

func (codexImageReadAfterCompleted) Read([]byte) (int, error) {
	return 0, errors.New("response body read after terminal event")
}

func TestBuildCodexImageBody(t *testing.T) {
	n := uint(2)
	request := dto.ImageRequest{
		Model:   "gpt-image-2",
		Prompt:  "a red circle",
		N:       &n,
		Size:    "1024x1024",
		Quality: "low",
	}
	body := buildCodexImageBody(request, "gpt-5.4", "generate", nil, "")

	assert.Equal(t, "gpt-5.4", body["model"])
	assert.Equal(t, true, body["stream"])
	assert.Equal(t, false, body["store"])
	tools := body["tools"].([]any)
	require.Len(t, tools, 1)
	tool := tools[0].(map[string]any)
	assert.Equal(t, "image_generation", tool["type"])
	assert.Equal(t, "generate", tool["action"])
	assert.Equal(t, "gpt-image-2", tool["model"])
	assert.Equal(t, "1024x1024", tool["size"])
	assert.Equal(t, "low", tool["quality"])
	assert.NotContains(t, tool, "n")
}

func TestBuildCodexImageBodyEditIncludesImagesAndMask(t *testing.T) {
	body := buildCodexImageBody(
		dto.ImageRequest{Model: "gpt-image-2", Prompt: "add a star"},
		"gpt-5.4",
		"edit",
		[]string{"data:image/png;base64,AAA"},
		"data:image/png;base64,MMM",
	)

	input := body["input"].([]any)
	message := input[0].(map[string]any)
	content := message["content"].([]any)
	require.Len(t, content, 2)
	assert.Equal(t, "input_image", content[1].(map[string]any)["type"])
	assert.Equal(t, "data:image/png;base64,AAA", content[1].(map[string]any)["image_url"])
	mask := body["tools"].([]any)[0].(map[string]any)["input_image_mask"].(map[string]any)
	assert.Equal(t, "data:image/png;base64,MMM", mask["image_url"])
}

func TestConvertImageRequestUsesCarrierAndForcesSSE(t *testing.T) {
	info := &relaycommon.RelayInfo{
		RelayMode: relayconstant.RelayModeImagesGenerations,
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelSetting: dto.ChannelSettings{ImageCarrierModel: "gpt-5.5"},
		},
	}

	converted, err := (&Adaptor{}).ConvertImageRequest(nil, info, dto.ImageRequest{
		Model:  "gpt-image-2",
		Prompt: "a red circle",
	})

	require.NoError(t, err)
	assert.True(t, info.IsStream)
	body := converted.(map[string]any)
	assert.Equal(t, "gpt-5.5", body["model"])
}

func TestConvertImageRequestJSONEditReadsAttachments(t *testing.T) {
	info := &relaycommon.RelayInfo{
		RelayMode:   relayconstant.RelayModeImagesEdits,
		ChannelMeta: &relaycommon.ChannelMeta{},
	}
	request := dto.ImageRequest{
		Model:  "gpt-image-2",
		Prompt: "edit this",
		Images: json.RawMessage(`["data:image/png;base64,AA==",{"url":"https://example.com/ref.png"}]`),
	}
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/images/edits", strings.NewReader(`{}`))
	c.Request.Header.Set("Content-Type", "application/json")

	converted, err := (&Adaptor{}).ConvertImageRequest(c, info, request)
	require.NoError(t, err)
	body := converted.(map[string]any)
	content := body["input"].([]any)[0].(map[string]any)["content"].([]any)
	require.Len(t, content, 3)
	assert.Equal(t, "data:image/png;base64,AA==", content[1].(map[string]any)["image_url"])
	assert.Equal(t, "https://example.com/ref.png", content[2].(map[string]any)["image_url"])
}

func TestConvertImageRequestMultipartEditReadsImageAndMask(t *testing.T) {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	require.NoError(t, writer.WriteField("model", "gpt-image-2"))
	require.NoError(t, writer.WriteField("prompt", "edit this"))
	imagePart, err := writer.CreateFormFile("image", "input.png")
	require.NoError(t, err)
	_, err = imagePart.Write([]byte("image-data"))
	require.NoError(t, err)
	maskPart, err := writer.CreateFormFile("mask", "mask.png")
	require.NoError(t, err)
	_, err = maskPart.Write([]byte("mask-data"))
	require.NoError(t, err)
	require.NoError(t, writer.Close())

	request := httptest.NewRequest(http.MethodPost, "/v1/images/edits", &body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	require.NoError(t, request.ParseMultipartForm(32<<20))
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = request
	info := &relaycommon.RelayInfo{
		RelayMode:   relayconstant.RelayModeImagesEdits,
		ChannelMeta: &relaycommon.ChannelMeta{},
	}

	converted, err := (&Adaptor{}).ConvertImageRequest(c, info, dto.ImageRequest{Model: "gpt-image-2", Prompt: "edit this"})
	require.NoError(t, err)
	content := converted.(map[string]any)["input"].([]any)[0].(map[string]any)["content"].([]any)
	require.Len(t, content, 2)
	assert.Equal(t, "data:image/png;base64,"+"aW1hZ2UtZGF0YQ==", content[1].(map[string]any)["image_url"])
	mask := converted.(map[string]any)["tools"].([]any)[0].(map[string]any)["input_image_mask"].(map[string]any)
	assert.Equal(t, "data:image/png;base64,"+"bWFzay1kYXRh", mask["image_url"])
}

func TestConvertImageRequestRejectsInvalidInputs(t *testing.T) {
	info := &relaycommon.RelayInfo{
		RelayMode:   relayconstant.RelayModeImagesGenerations,
		ChannelMeta: &relaycommon.ChannelMeta{},
	}

	_, err := (&Adaptor{}).ConvertImageRequest(nil, info, dto.ImageRequest{Model: "gpt-image-2", ResponseFormat: "url"})
	var apiErr *types.NewAPIError
	require.ErrorAs(t, err, &apiErr)
	assert.Equal(t, http.StatusBadRequest, apiErr.StatusCode)

	assert.Error(t, ValidateCodexImageRequest(dto.ImageRequest{ResponseFormat: "url"}))
	assert.NoError(t, ValidateCodexImageRequest(dto.ImageRequest{ResponseFormat: "b64_json"}))
}

func TestReadCodexJSONEditImagesValidatesEntries(t *testing.T) {
	validInline := "data:image/png;base64," + base64.StdEncoding.EncodeToString(make([]byte, 1024))
	oversizedInline := "data:image/png;base64," + strings.Repeat("A", base64.StdEncoding.EncodedLen(codexImageInputSizeLimit+1))
	tooMany := make([]string, codexImageInputCountLimit+1)
	for index := range tooMany {
		tooMany[index] = fmt.Sprintf("https://example.com/reference-%d.png", index)
	}

	testCases := []struct {
		name          string
		entries       []string
		errorContains string
	}{
		{name: "empty", entries: []string{}, errorContains: "image is required"},
		{name: "too many", entries: tooMany, errorContains: "at most 16 reference images"},
		{name: "oversized inline image", entries: []string{oversizedInline}, errorContains: "exceeds the 50 MiB image limit"},
		{name: "non image data URL", entries: []string{"data:text/plain;base64,SGVsbG8="}, errorContains: "image data URL"},
		{name: "unsupported image MIME", entries: []string{"data:image/svg+xml;base64,PHN2Zy8+"}, errorContains: "unsupported image type"},
		{name: "valid inline image", entries: []string{validInline}},
		{name: "valid HTTPS image", entries: []string{"https://example.com/reference.png"}},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			raw, err := common.Marshal(testCase.entries)
			require.NoError(t, err)
			images, _, err := readCodexJSONEditImages(dto.ImageRequest{Images: json.RawMessage(raw)})
			if testCase.errorContains != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), testCase.errorContains)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, testCase.entries, images)
		})
	}
}

func TestReadCodexEditImagesRejectsTooManyFiles(t *testing.T) {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	for index := 0; index < codexImageInputCountLimit+1; index++ {
		part, err := writer.CreateFormFile("image[]", fmt.Sprintf("reference-%d.png", index))
		require.NoError(t, err)
		_, err = part.Write([]byte("image"))
		require.NoError(t, err)
	}
	require.NoError(t, writer.Close())

	request := httptest.NewRequest(http.MethodPost, "/v1/images/edits", &body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	require.NoError(t, request.ParseMultipartForm(1<<20))
	requestContext, _ := gin.CreateTestContext(httptest.NewRecorder())
	requestContext.Request = request

	_, _, err := readCodexEditImages(requestContext)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "at most 16 reference images")
}

func TestRelayImageOverCodexParsesImageAndUsage(t *testing.T) {
	sse := strings.Join([]string{
		`data: {"type":"response.output_item.done","item":{"type":"image_generation_call","result":"QUJD","revised_prompt":"a red circle"}}`,
		`data: {"type":"response.completed","response":{"created_at":1700000000,"tool_usage":{"image_gen":{"input_tokens":21,"output_tokens":196,"total_tokens":217,"input_tokens_details":{"image_tokens":4},"output_tokens_details":{"image_tokens":196}}}}}`,
		"data: [DONE]",
		"",
	}, "\n\n")
	response := &http.Response{StatusCode: http.StatusOK, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(sse))}
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)

	usage, apiErr := RelayImageOverCodex(c, &relaycommon.RelayInfo{}, response)
	require.Nil(t, apiErr)
	require.NotNil(t, usage)
	assert.Equal(t, 21, usage.PromptTokens)
	assert.Equal(t, 196, usage.CompletionTokens)
	assert.Equal(t, 217, usage.TotalTokens)
	assert.Equal(t, 4, usage.PromptTokensDetails.ImageTokens)
	assert.Equal(t, 196, usage.CompletionTokenDetails.ImageTokens)

	var imageResponse dto.ImageResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &imageResponse))
	require.Len(t, imageResponse.Data, 1)
	assert.Equal(t, "QUJD", imageResponse.Data[0].B64Json)
	assert.Equal(t, "a red circle", imageResponse.Data[0].RevisedPrompt)
}

func TestRelayImageOverCodexUsesDeliveredImageCount(t *testing.T) {
	requestedCount := 2
	testCases := []struct {
		name    string
		newInfo func() *relaycommon.RelayInfo
		verify  func(*testing.T, *relaycommon.RelayInfo)
	}{
		{
			name: "tiered expression",
			newInfo: func() *relaycommon.RelayInfo {
				return &relaycommon.RelayInfo{TieredBillingSnapshot: &billingexpr.BillingSnapshot{EstimatedImageCount: &requestedCount}}
			},
			verify: func(t *testing.T, info *relaycommon.RelayInfo) {
				require.NotNil(t, info.BillingImageCount)
				assert.Equal(t, 1, *info.BillingImageCount)
			},
		},
		{
			name: "fixed price",
			newInfo: func() *relaycommon.RelayInfo {
				priceData := hosttypes.PriceData{UsePrice: true}
				priceData.AddOtherRatio("n", 2)
				return &relaycommon.RelayInfo{PriceData: priceData}
			},
			verify: func(t *testing.T, info *relaycommon.RelayInfo) {
				assert.Nil(t, info.BillingImageCount)
				assert.Equal(t, 1.0, info.PriceData.OtherRatios()["n"])
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			info := testCase.newInfo()
			sse := "data: {\"type\":\"response.output_item.done\",\"item\":{\"type\":\"image_generation_call\",\"result\":\"QUJD\"}}\n\n" +
				"data: {\"type\":\"response.completed\",\"response\":{\"created_at\":1700000000}}\n\n"
			recorder := httptest.NewRecorder()
			requestContext, _ := gin.CreateTestContext(recorder)
			_, apiError := RelayImageOverCodex(requestContext, info, &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(sse)),
			})
			require.Nil(t, apiError)
			testCase.verify(t, info)
		})
	}
}

func TestRelayImageOverCodexStopsAtCompleted(t *testing.T) {
	payload := "data: {\"type\":\"response.output_item.done\",\"item\":{\"type\":\"image_generation_call\",\"result\":\"QUJD\"}}\n\n" +
		"data: {\"type\":\"response.completed\",\"response\":{\"created_at\":1700000000}}\n\n"
	recorder := httptest.NewRecorder()
	requestContext, _ := gin.CreateTestContext(recorder)
	_, apiError := RelayImageOverCodex(requestContext, &relaycommon.RelayInfo{}, &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(io.MultiReader(strings.NewReader(payload), codexImageReadAfterCompleted{})),
	})
	require.Nil(t, apiError)
	assert.Equal(t, http.StatusOK, recorder.Code)
}

func TestRelayImageOverCodexRejectsBareErrorEvent(t *testing.T) {
	payload := "data: {\"type\":\"response.output_item.done\",\"item\":{\"type\":\"image_generation_call\",\"result\":\"QUJD\"}}\n\n" +
		"data: {\"type\":\"error\",\"code\":\"server_error\",\"message\":\"generation aborted\"}\n\n"
	recorder := httptest.NewRecorder()
	requestContext, _ := gin.CreateTestContext(recorder)
	_, apiError := RelayImageOverCodex(requestContext, &relaycommon.RelayInfo{}, &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(payload)),
	})
	require.NotNil(t, apiError)
	assert.False(t, requestContext.Writer.Written())
}

func TestRelayImageOverCodexIgnoresEventsAfterCompleted(t *testing.T) {
	sse := strings.Join([]string{
		`data: {"type":"response.output_item.done","item":{"type":"image_generation_call","result":"QUJD"}}`,
		`data: {"type":"response.completed","response":{"created_at":1700000000}}`,
		`data: {"type":"response.output_item.done","item":{"type":"image_generation_call","result":"REVG"}}`,
		"",
	}, "\n\n")
	recorder := httptest.NewRecorder()
	requestContext, _ := gin.CreateTestContext(recorder)
	_, apiError := RelayImageOverCodex(requestContext, &relaycommon.RelayInfo{}, &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(sse)),
	})
	require.Nil(t, apiError)
	var response dto.ImageResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	require.Len(t, response.Data, 1)
	assert.Equal(t, "QUJD", response.Data[0].B64Json)
}

func TestRelayImageOverCodexDoesNotGuessUsage(t *testing.T) {
	sse := strings.Join([]string{
		`data: {"type":"response.output_item.done","item":{"type":"image_generation_call","result":"QUJD"}}`,
		`data: {"type":"response.completed","response":{"created_at":1700000000}}`,
		"",
	}, "\n\n")
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	usage, apiErr := RelayImageOverCodex(c, &relaycommon.RelayInfo{}, &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(sse)),
	})

	require.Nil(t, apiErr)
	require.NotNil(t, usage)
	assert.Zero(t, usage.TotalTokens)
	assert.Zero(t, usage.CompletionTokens)
}

func TestRelayImageOverCodexFailedEventIsGeneric(t *testing.T) {
	sse := `data: {"type":"response.failed","response":{"error":{"message":"ChatGPT internal model details"}}}`
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	usage, apiErr := RelayImageOverCodex(c, &relaycommon.RelayInfo{}, &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(sse)),
	})

	require.Nil(t, usage)
	require.NotNil(t, apiErr)
	assert.Contains(t, apiErr.Error(), "codex image generation failed")
	assert.NotContains(t, apiErr.Error(), "ChatGPT internal")
}

func TestGetRequestURLImageUsesResponsesEndpoint(t *testing.T) {
	info := &relaycommon.RelayInfo{
		RelayMode: relayconstant.RelayModeImagesGenerations,
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelType:    constant.ChannelTypeCodex,
			ChannelBaseUrl: "https://chatgpt.com",
		},
	}
	url, err := (&Adaptor{}).GetRequestURL(info)
	require.NoError(t, err)
	assert.Equal(t, "https://chatgpt.com/backend-api/codex/responses", url)
}

func TestResolveImageCarrierModelFallsBackSafely(t *testing.T) {
	original := model_setting.GetCodexSettings().ImageCarrierModel
	settings := config.GlobalConfig.Get("codex")
	t.Cleanup(func() {
		require.NoError(t, config.UpdateConfigFromMap(settings, map[string]string{"image_carrier_model": original}))
	})
	require.NoError(t, config.UpdateConfigFromMap(settings, map[string]string{"image_carrier_model": ""}))
	assert.Equal(t, defaultImageCarrierModel, resolveImageCarrierModel(&relaycommon.RelayInfo{}))
	require.NoError(t, config.UpdateConfigFromMap(settings, map[string]string{"image_carrier_model": "gpt-global"}))
	assert.Equal(t, "gpt-global", resolveImageCarrierModel(&relaycommon.RelayInfo{}))
}

func TestValidateCodexImageReference(t *testing.T) {
	for _, value := range []string{"data:image/png;base64,AA==", "https://example.com/a.png", "http://example.com/a.png"} {
		assert.NoError(t, validateCodexImageReference(value))
	}
	assert.Error(t, validateCodexImageReference("file:///etc/passwd"))
}

func TestFileHeaderToDataURLRejectsUnsupportedType(t *testing.T) {
	fh := &multipart.FileHeader{Filename: "payload.txt", Size: 1}
	_, err := fileHeaderToDataURL(fh)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported image type")
}

func TestFileHeaderToDataURLRejectsOversizedFile(t *testing.T) {
	fileHeader := &multipart.FileHeader{Filename: "oversized.png", Size: codexImageInputSizeLimit + 1}
	_, err := fileHeaderToDataURL(fileHeader)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "exceeds the 50 MiB image limit")
}
