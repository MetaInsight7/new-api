package controller

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
)

const modelAvailabilityProbePrompt = "我发出'ping'命令，请你只回我'pong'"

type modelProbeClass string

const (
	modelProbeAvailable           modelProbeClass = "available"
	modelProbeTemporaryFailure    modelProbeClass = "temporary_failure"
	modelProbeOfficialUnsupported modelProbeClass = "official_unsupported"
	modelProbeUnknownFailure      modelProbeClass = "unknown_failure"
)

type modelProbeOutcome struct {
	Class      modelProbeClass
	ReasonType string
	Message    string
	ChannelID  int
}

type modelAvailabilityTaskSummary struct {
	Total     int `json:"total"`
	Processed int `json:"processed"`
	Available int `json:"available"`
	Failed    int `json:"failed"`
}

type modelAvailabilityHandler struct{}

func (modelAvailabilityHandler) Type() string { return model.SystemTaskTypeModelAvailability }

func (modelAvailabilityHandler) Enabled() bool {
	return common.GetEnvOrDefaultBool("MODEL_AVAILABILITY_CHECK_ENABLED", true)
}

func (modelAvailabilityHandler) Interval() time.Duration {
	seconds := common.GetEnvOrDefault("MODEL_AVAILABILITY_CHECK_INTERVAL_SECONDS", 86400)
	if seconds < 1 {
		seconds = 86400
	}
	return time.Duration(seconds) * time.Second
}

func modelAvailabilityUnsupportedThreshold() int {
	threshold := common.GetEnvOrDefault("MODEL_AVAILABILITY_OFFICIAL_UNSUPPORTED_THRESHOLD", 2)
	if threshold < 1 {
		return 1
	}
	return threshold
}

func modelAvailabilityProbeInterval() time.Duration {
	seconds := common.GetEnvOrDefault("MODEL_AVAILABILITY_PROBE_INTERVAL_SECONDS", 60)
	if seconds < 1 {
		seconds = 60
	}
	return time.Duration(seconds) * time.Second
}

func (modelAvailabilityHandler) NewPayload() any { return nil }

func (modelAvailabilityHandler) Run(ctx context.Context, task *model.SystemTask, runnerID string) {
	summary, err := runModelAvailabilityDetection(ctx, service.NewSystemTaskProgressReporter(task, runnerID))
	if err != nil {
		if errors.Is(err, context.Canceled) {
			if cancelled, cancelErr := model.IsSystemTaskCancellationRequested(task.TaskID, runnerID); cancelErr == nil && cancelled {
				err = errors.New("model availability detection cancelled")
			}
		}
		finishSystemTaskHandler(task, runnerID, model.SystemTaskStatusFailed, summary, err)
		return
	}
	finishSystemTaskHandler(task, runnerID, model.SystemTaskStatusSucceeded, summary, nil)
}

func classifyModelProbeError(err error, apiErr *types.NewAPIError) modelProbeOutcome {
	statusCode := 0
	if apiErr != nil {
		statusCode = apiErr.StatusCode
	}
	message := ""
	if err != nil {
		message = err.Error()
	} else if apiErr != nil {
		message = apiErr.Error()
	}
	lower := strings.ToLower(message)

	for _, pattern := range []string{
		"invalid api key", "invalid credential", "unauthorized", "not authorized",
		"permission denied", "access denied", "forbidden", "insufficient balance",
		"insufficient quota", "quota exceeded", "exceeded quota", "billing",
		"no available key", "do not have access", "for your account", "for this account",
	} {
		if strings.Contains(lower, pattern) {
			return modelProbeOutcome{Class: modelProbeUnknownFailure, ReasonType: "channel_account_issue", Message: message}
		}
	}
	if statusCode == http.StatusUnauthorized || statusCode == http.StatusForbidden {
		return modelProbeOutcome{Class: modelProbeUnknownFailure, ReasonType: "channel_account_issue", Message: message}
	}
	if statusCode == http.StatusTooManyRequests || statusCode == http.StatusRequestTimeout ||
		(statusCode >= http.StatusInternalServerError && statusCode <= 599) ||
		strings.Contains(lower, "timeout") || strings.Contains(lower, "temporarily unavailable") ||
		strings.Contains(lower, "service unavailable") || strings.Contains(lower, "overloaded") ||
		strings.Contains(lower, "rate limit") {
		return modelProbeOutcome{Class: modelProbeTemporaryFailure, ReasonType: "temporary_upstream_failure", Message: message}
	}

	for _, pattern := range []string{
		"model is not supported", "model not supported", "not supported when using",
		"unsupported model", "model_not_found", "has been deprecated", "has been retired", "sunset",
	} {
		if strings.Contains(lower, pattern) {
			return modelProbeOutcome{Class: modelProbeOfficialUnsupported, ReasonType: "official_model_unsupported", Message: message}
		}
	}
	if statusCode == http.StatusNotFound && (strings.Contains(lower, "model not found") || strings.Contains(lower, "model does not exist") || strings.Contains(lower, "invalid model")) {
		return modelProbeOutcome{Class: modelProbeOfficialUnsupported, ReasonType: "official_model_unsupported", Message: message}
	}
	if strings.Contains(lower, "model does not exist") {
		return modelProbeOutcome{Class: modelProbeOfficialUnsupported, ReasonType: "official_model_unsupported", Message: message}
	}
	return modelProbeOutcome{Class: modelProbeUnknownFailure, ReasonType: "unknown_probe_failure", Message: message}
}

func modelProbeOutcomeForUntestableChannels() modelProbeOutcome {
	return modelProbeOutcome{Class: modelProbeAvailable, ReasonType: "no_testable_channel"}
}

func shouldPersistModelProbeOutcome(outcome modelProbeOutcome) bool {
	return outcome.ReasonType != "no_testable_channel"
}

type modelProbeModality int

const (
	modelProbeModalityChat modelProbeModality = iota
	modelProbeModalityImage
	modelProbeModalityEmbedding
	modelProbeModalityRerank
	modelProbeModalityUntestable
)

var untestableProbeChannelTypes = map[int]bool{
	constant.ChannelTypeMidjourney:     true,
	constant.ChannelTypeMidjourneyPlus: true,
	constant.ChannelTypeSunoAPI:        true,
	constant.ChannelTypeKling:          true,
	constant.ChannelTypeJimeng:         true,
	constant.ChannelTypeDoubaoVideo:    true,
	constant.ChannelTypeVidu:           true,
	constant.ChannelTypeSora:           true,
}

var untestableProbeModelSubstrings = []string{
	"tts", "whisper", "audio", "speech", "voice", "transcrib", "realtime",
	"moderation", "video", "sora", "veo", "seedance", "runway", "sound", "music",
}

var imageProbeModelSubstrings = []string{
	"seedream", "stable-diffusion", "sd3", "sdxl", "recraft", "ideogram", "kolors",
	"nano-banana", "hunyuan-image", "qwen-image", "wanx", "playground-v", "kontext",
}

var embeddingProbeModelSubstrings = []string{"embedding", "embed", "bge-", "m3e", "gte-", "text2vec"}

func classifyModelProbeModality(channelType int, modelName string) modelProbeModality {
	lower := strings.ToLower(strings.TrimSpace(modelName))
	if untestableProbeChannelTypes[channelType] {
		return modelProbeModalityUntestable
	}
	for _, value := range untestableProbeModelSubstrings {
		if strings.Contains(lower, value) {
			return modelProbeModalityUntestable
		}
	}
	if strings.Contains(lower, "rerank") {
		return modelProbeModalityRerank
	}
	for _, value := range embeddingProbeModelSubstrings {
		if strings.Contains(lower, value) {
			return modelProbeModalityEmbedding
		}
	}
	if channelType == constant.ChannelTypeJina {
		return modelProbeModalityRerank
	}
	if common.IsImageGenerationModel(modelName) {
		return modelProbeModalityImage
	}
	for _, value := range imageProbeModelSubstrings {
		if strings.Contains(lower, value) {
			return modelProbeModalityImage
		}
	}
	return modelProbeModalityChat
}

func modelAvailabilityProbeConfig(modelName string, channelType int) (string, channelTestOptions, bool) {
	options := channelTestOptions{
		Prompt:     modelAvailabilityProbePrompt,
		ExpectPong: true,
		TokenName:  "模型可用性检测",
		LogContent: "模型可用性检测",
		MaxTokens:  8,
		SkipLog:    true,
	}
	switch classifyModelProbeModality(channelType, modelName) {
	case modelProbeModalityUntestable:
		return "", options, false
	case modelProbeModalityImage:
		options.ExpectPong = false
		return string(constant.EndpointTypeImageGeneration), options, true
	case modelProbeModalityEmbedding:
		options.ExpectPong = false
		return string(constant.EndpointTypeEmbeddings), options, true
	case modelProbeModalityRerank:
		options.ExpectPong = false
		return string(constant.EndpointTypeJinaRerank), options, true
	default:
		return "", options, true
	}
}

func saveModelAvailabilityProbeResult(modelName string, outcome modelProbeOutcome) error {
	now := common.GetTimestamp()
	state := &model.ModelAvailabilityState{
		ModelName:     modelName,
		Status:        string(outcome.Class),
		ReasonType:    outcome.ReasonType,
		Reason:        buildModelAvailabilityReason(outcome),
		LastError:     outcome.Message,
		LastCheckedAt: now,
	}
	existing, err := model.GetModelAvailabilityState(modelName)
	if err != nil {
		return err
	}
	if outcome.Class == modelProbeAvailable {
		state.Status = model.ModelAvailabilityAvailable
		state.ReasonType = ""
		state.Reason = ""
		state.LastError = ""
		state.FirstDetectedAt = 0
		state.LastSuccessAt = now
		state.ConsecutiveFailures = 0
		return model.SaveModelAvailabilityState(state)
	}
	state.FirstDetectedAt = now
	state.ConsecutiveFailures = 1
	if existing != nil {
		if existing.FirstDetectedAt > 0 {
			state.FirstDetectedAt = existing.FirstDetectedAt
		}
		if existing.ReasonType == outcome.ReasonType || (outcome.Class == modelProbeOfficialUnsupported && existing.ReasonType == "official_model_unsupported_candidate") {
			state.ConsecutiveFailures = existing.ConsecutiveFailures + 1
		}
		state.LastSuccessAt = existing.LastSuccessAt
	}
	if outcome.Class == modelProbeOfficialUnsupported && state.ConsecutiveFailures < modelAvailabilityUnsupportedThreshold() {
		state.Status = model.ModelAvailabilityUnknownFailure
		state.ReasonType = "official_model_unsupported_candidate"
	}
	return model.SaveModelAvailabilityState(state)
}

func buildModelAvailabilityReason(outcome modelProbeOutcome) string {
	switch outcome.Class {
	case modelProbeOfficialUnsupported:
		return "上游明确返回该模型当前不受支持，模型可能已被官方下线或当前账号类型不再支持。"
	case modelProbeTemporaryFailure:
		return "检测时上游出现临时故障，暂不判定为模型下线。"
	default:
		return "检测失败但无法确认是官方下线，建议查看渠道、账号权限或上游错误。"
	}
}

func summarizeModelProbeOutcomes(outcomes []modelProbeOutcome, sawUntestable bool) modelProbeOutcome {
	if len(outcomes) == 0 {
		if sawUntestable {
			return modelProbeOutcomeForUntestableChannels()
		}
		return modelProbeOutcome{Class: modelProbeUnknownFailure, ReasonType: "empty_probe_result"}
	}
	allOfficialUnsupported := true
	var temporary, unknown *modelProbeOutcome
	for index := range outcomes {
		if outcomes[index].Class != modelProbeOfficialUnsupported {
			allOfficialUnsupported = false
		}
		if outcomes[index].Class == modelProbeTemporaryFailure && temporary == nil {
			temporary = &outcomes[index]
		}
		if outcomes[index].Class == modelProbeUnknownFailure && unknown == nil {
			unknown = &outcomes[index]
		}
	}
	if allOfficialUnsupported {
		return outcomes[0]
	}
	if temporary != nil {
		return *temporary
	}
	if unknown != nil {
		return *unknown
	}
	return outcomes[0]
}

func probeOneModelAvailability(ctx context.Context, modelName string, testUserID int) (modelProbeOutcome, error) {
	targets, err := model.GetModelAvailabilityProbeTargets(modelName)
	if err != nil {
		return modelProbeOutcome{Class: modelProbeUnknownFailure, ReasonType: "probe_target_query_failed", Message: err.Error()}, err
	}
	if len(targets) == 0 {
		outcome := modelProbeOutcome{Class: modelProbeUnknownFailure, ReasonType: "no_available_channel", Message: "no enabled channel found for model"}
		return outcome, saveModelAvailabilityProbeResult(modelName, outcome)
	}
	outcomes := make([]modelProbeOutcome, 0, len(targets))
	sawUntestable := false
	for _, target := range targets {
		if err := ctx.Err(); err != nil {
			return modelProbeOutcome{}, err
		}
		channel, loadErr := model.GetChannelById(target.ChannelID, true)
		if loadErr != nil {
			outcomes = append(outcomes, modelProbeOutcome{Class: modelProbeUnknownFailure, ReasonType: "channel_load_failed", Message: loadErr.Error(), ChannelID: target.ChannelID})
			continue
		}
		endpointType, options, testable := modelAvailabilityProbeConfig(modelName, channel.Type)
		if !testable {
			sawUntestable = true
			continue
		}
		options.Context = ctx
		result := testChannelWithOptions(channel, testUserID, modelName, endpointType, false, options)
		if result.localErr == nil && result.newAPIError == nil {
			outcome := modelProbeOutcome{Class: modelProbeAvailable}
			return outcome, model.SaveModelAvailabilityState(&model.ModelAvailabilityState{ModelName: modelName, Status: model.ModelAvailabilityAvailable, LastCheckedAt: common.GetTimestamp(), LastSuccessAt: common.GetTimestamp()})
		}
		outcome := classifyModelProbeError(result.localErr, result.newAPIError)
		outcome.ChannelID = target.ChannelID
		outcomes = append(outcomes, outcome)
	}
	final := summarizeModelProbeOutcomes(outcomes, sawUntestable)
	if !shouldPersistModelProbeOutcome(final) {
		return final, nil
	}
	return final, saveModelAvailabilityProbeResult(modelName, final)
}

func runModelAvailabilityDetection(ctx context.Context, report func(processed, total int)) (modelAvailabilityTaskSummary, error) {
	return runModelAvailabilityDetectionWithInterval(ctx, report, modelAvailabilityProbeInterval())
}

func runModelAvailabilityDetectionWithInterval(ctx context.Context, report func(processed, total int), probeInterval time.Duration) (modelAvailabilityTaskSummary, error) {
	testUserID, err := resolveChannelTestUserID(nil)
	if err != nil {
		return modelAvailabilityTaskSummary{}, err
	}
	modelNames, err := model.GetModelAvailabilityProbeModelNames()
	if err != nil {
		return modelAvailabilityTaskSummary{}, err
	}
	summary := modelAvailabilityTaskSummary{Total: len(modelNames)}
	if report != nil {
		report(0, summary.Total)
	}
	for index, modelName := range modelNames {
		if err := ctx.Err(); err != nil {
			return summary, err
		}
		outcome, probeErr := probeOneModelAvailability(ctx, modelName, testUserID)
		if probeErr != nil {
			if errors.Is(probeErr, context.Canceled) || errors.Is(probeErr, context.DeadlineExceeded) {
				return summary, probeErr
			}
			common.SysError(fmt.Sprintf("model availability probe failed for %s: %v", modelName, probeErr))
		}
		if outcome.Class == modelProbeAvailable {
			summary.Available++
		} else {
			summary.Failed++
		}
		summary.Processed = index + 1
		if report != nil {
			report(summary.Processed, summary.Total)
		}
		if index < len(modelNames)-1 {
			if err := waitForModelAvailabilityProbeInterval(ctx, probeInterval); err != nil {
				return summary, err
			}
		}
	}
	model.InvalidatePricingCache()
	return summary, nil
}

func waitForModelAvailabilityProbeInterval(ctx context.Context, interval time.Duration) error {
	if interval <= 0 {
		return nil
	}
	timer := time.NewTimer(interval)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

// TriggerModelAvailabilityDetection enqueues a leased system task instead of
// starting a process-local goroutine. This keeps manual runs single-flight on
// every deployment topology and exposes the run in System Info.
func TriggerModelAvailabilityDetection(c *gin.Context) {
	task, created, err := service.EnqueueSystemTask(model.SystemTaskTypeModelAvailability, nil)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if !created {
		c.JSON(http.StatusConflict, gin.H{"success": false, "message": "model availability detection is already running", "data": task.ToResponse()})
		return
	}
	common.ApiSuccess(c, gin.H{"task_id": task.TaskID, "status": task.Status})
}
