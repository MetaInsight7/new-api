package controller

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relay/channel/openai"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-gonic/gin"
)

const CodexModelGovernanceProbeTaskType = "codex_model_governance_probe"
const CodexModelGovernanceNoticeTaskType = "codex_model_governance_notice"

type codexGovernanceTaskHandler struct {
	notice bool
}

func (handler codexGovernanceTaskHandler) Type() string {
	if handler.notice {
		return CodexModelGovernanceNoticeTaskType
	}
	return CodexModelGovernanceProbeTaskType
}

func (handler codexGovernanceTaskHandler) Enabled() bool {
	setting := operation_setting.GetCodexModelGovernanceSetting()
	if handler.notice {
		return setting.Enabled && len(setting.OfficialSourceURLs) > 0
	}
	return setting.Enabled && setting.ProbeEnabled
}

func (handler codexGovernanceTaskHandler) Interval() time.Duration {
	return time.Duration(max(60, operation_setting.GetCodexModelGovernanceSetting().ProbeIntervalMinutes)) * time.Minute
}

func (handler codexGovernanceTaskHandler) NewPayload() any { return nil }

func (handler codexGovernanceTaskHandler) Run(ctx context.Context, task *model.SystemTask, runnerID string) {
	processed := 0
	report := service.NewSystemTaskProgressReporter(task, runnerID)
	var runErr error
	if handler.notice {
		processed, runErr = runCodexGovernanceNoticeMonitor(ctx, report)
	} else {
		processed, runErr = runCodexGovernanceProbes(ctx, report)
	}
	status := model.SystemTaskStatusSucceeded
	if runErr != nil {
		status = model.SystemTaskStatusFailed
		runErr = fmt.Errorf("%s", model.SanitizeCodexModelGovernanceEvidence(runErr.Error()))
	}
	finishSystemTaskHandler(task, runnerID, status, map[string]int{"processed": processed}, runErr)
}

func StartCodexModelGovernanceTask() {
	service.RegisterSystemTaskHandler(codexGovernanceTaskHandler{})
	service.RegisterSystemTaskHandler(codexGovernanceTaskHandler{notice: true})
}

func TriggerCodexModelGovernanceTask(c *gin.Context) {
	notice := c.Query("source") == "notice"
	handler := codexGovernanceTaskHandler{notice: notice}
	if !handler.Enabled() {
		common.ApiErrorMsg(c, "Codex governance task is disabled or unconfigured")
		return
	}
	task, created, err := service.EnqueueSystemTask(handler.Type(), nil)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{"task_id": task.TaskID, "status": task.Status, "created": created})
}

func runCodexGovernanceProbes(ctx context.Context, report func(int, int)) (int, error) {
	handler := codexGovernanceTaskHandler{}
	if !handler.Enabled() {
		return 0, nil
	}
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	testUserID, err := resolveChannelTestUserID(nil)
	if err != nil {
		return 0, err
	}
	channels, err := model.GetCodexModelGovernanceChannels(ctx, true)
	if err != nil {
		return 0, err
	}
	total := 0
	modelsByChannel := make(map[int][]string, len(channels))
	for _, channel := range channels {
		if channel.Status != common.ChannelStatusEnabled {
			continue
		}
		seen := make(map[string]struct{})
		for _, modelName := range channel.GetModels() {
			modelName = strings.TrimSpace(modelName)
			if modelName == "" {
				continue
			}
			if _, exists := seen[modelName]; exists {
				continue
			}
			seen[modelName] = struct{}{}
			modelsByChannel[channel.Id] = append(modelsByChannel[channel.Id], modelName)
			total++
		}
	}
	processed := 0
	if report != nil {
		report(0, total)
	}
	for _, channel := range channels {
		if channel.Status != common.ChannelStatusEnabled {
			continue
		}
		for _, configuredModel := range modelsByChannel[channel.Id] {
			if err := ctx.Err(); err != nil {
				return processed, err
			}
			if !handler.Enabled() {
				return processed, nil
			}
			modelName := strings.TrimSpace(configuredModel)
			if modelName == "" {
				continue
			}
			probeCtx, cancel := context.WithTimeout(ctx, time.Minute)
			patterns := operation_setting.GetCodexModelGovernanceSetting().UnsupportedMessagePatterns
			var probeMatch service.CodexUnsupportedMatch
			probeCtx = openai.WithCodexModelProbeErrorEvidence(probeCtx, func(message string) {
				probeMatch = service.ClassifyCodexUnsupportedMessage(message, patterns)
			})
			result := testChannelWithOptions(channel, testUserID, modelName, string(constant.EndpointTypeOpenAIResponse), true, channelTestOptions{
				Prompt: "ping", ExpectPong: true, MaxTokens: 8, SkipLog: true,
				TokenName: "Codex model governance probe", LogContent: "Codex model governance probe", Context: probeCtx,
			})
			cancel()
			if err := ctx.Err(); err != nil {
				return processed, err
			}
			if !handler.Enabled() {
				return processed, nil
			}
			if result.localErr == nil && result.newAPIError == nil {
				err = model.ResetCodexModelGovernanceProbeFailure(modelName, channel.Id)
				if err == nil {
					err = model.RestoreCodexModelGovernanceAfterProbeSuccess(modelName, channel.Id)
				}
			} else {
				message := ""
				if result.localErr != nil {
					message = result.localErr.Error()
				} else {
					message = result.newAPIError.Error()
				}
				match := probeMatch
				if !match.Matched {
					match = service.ClassifyCodexUnsupportedMessage(message, patterns)
				}
				if !match.Matched {
					err = model.ResetCodexModelGovernanceProbeFailure(modelName, channel.Id)
				} else {
					var confirmed bool
					_, confirmed, err = model.RecordCodexModelGovernanceProbeUnsupportedFailure(modelName, channel.Id, 2)
					if err == nil && confirmed {
						_, err = service.MoveCodexModelToPendingReviewWithContext(ctx, service.CodexModelUnsupportedFinding{
							ModelName: modelName, Source: model.CodexModelGovernanceSourceProbe, MatchedRule: match.Pattern,
							LastError: fmt.Sprintf("Codex probe matched a configured unsupported-model rule for %q", modelName), AffectedChannelIDs: []int{channel.Id},
						})
						if err == nil {
							err = model.ResetCodexModelGovernanceProbeFailure(modelName, channel.Id)
						}
					}
				}
			}
			if err != nil {
				return processed, err
			}
			processed++
			if report != nil {
				report(processed, total)
			}
		}
	}
	return processed, nil
}

func runCodexGovernanceNoticeMonitor(ctx context.Context, report func(int, int)) (int, error) {
	if !(codexGovernanceTaskHandler{notice: true}).Enabled() {
		return 0, nil
	}
	channels, err := model.GetCodexModelGovernanceChannels(ctx, false)
	if err != nil {
		return 0, err
	}
	candidates := make([]string, 0)
	for _, channel := range channels {
		candidates = append(candidates, channel.GetModels()...)
	}
	setting := operation_setting.GetCodexModelGovernanceSetting()
	urls := append([]string(nil), setting.OfficialSourceURLs...)
	processed := 0
	var fetchErrors []error
	if report != nil {
		report(0, len(urls))
	}
	for _, sourceURL := range urls {
		if err := ctx.Err(); err != nil {
			return processed, err
		}
		if !(codexGovernanceTaskHandler{notice: true}).Enabled() {
			return processed, nil
		}
		body, err := service.FetchCodexOfficialSource(ctx, sourceURL)
		if err != nil {
			if ctx.Err() != nil {
				return processed, ctx.Err()
			}
			fetchErrors = append(fetchErrors, errors.New("official source fetch failed"))
			processed++
			if report != nil {
				report(processed, len(urls))
			}
			continue
		}
		analysis := operation_setting.GetMonitorAIAnalysisConfig()
		findings, _, analysisErr := service.ExtractCodexOfficialNoticeFindingsWithOptionalAIWithOptions(ctx, body, candidates, setting.OfficialLifecycleTerms, sourceURL, service.CodexOfficialNoticeAIOptions{
			APIKey: analysis.APIKey, BaseURL: analysis.BaseURL, Model: analysis.Model,
		})
		if analysisErr != nil {
			common.SysError("Codex notice AI analysis downgraded: " + model.SanitizeCodexModelGovernanceEvidence(analysisErr.Error()))
		}
		if err := ctx.Err(); err != nil {
			return processed, err
		}
		if !(codexGovernanceTaskHandler{notice: true}).Enabled() {
			return processed, nil
		}
		for _, finding := range findings {
			if _, err := service.MoveCodexModelToPendingReviewWithContext(ctx, finding); err != nil {
				return processed, err
			}
		}
		processed++
		if report != nil {
			report(processed, len(urls))
		}
	}
	return processed, errors.Join(fetchErrors...)
}
