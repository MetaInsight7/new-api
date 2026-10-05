package codex

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/setting/model_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
	t.Cleanup(func() { model_setting.GetCodexSettings().ImageCarrierModel = original })
	model_setting.GetCodexSettings().ImageCarrierModel = ""
	assert.Equal(t, defaultImageCarrierModel, resolveImageCarrierModel(&relaycommon.RelayInfo{}))
	model_setting.GetCodexSettings().ImageCarrierModel = "gpt-global"
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
