package perfmetrics

import (
	"context"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/types"
)

type Outcome string

const (
	OutcomeSuccess Outcome = "success"
	OutcomeFailure Outcome = "failure"
)

func ClassifyRelayOutcome(ctx context.Context, info *relaycommon.RelayInfo, apiErr *types.NewAPIError) Outcome {
	if info == nil || apiErr != nil || info.PerformanceBusinessRejection {
		return OutcomeFailure
	}
	if ctx != nil && ctx.Err() != nil {
		return OutcomeFailure
	}
	if info.StreamStatus == nil {
		return OutcomeSuccess
	}
	stream := info.StreamStatus.OutcomeSnapshot()
	if stream.HasErrors {
		return OutcomeFailure
	}
	switch stream.EndReason {
	case relaycommon.StreamEndReasonDone, relaycommon.StreamEndReasonEOF, relaycommon.StreamEndReasonHandlerStop:
	case relaycommon.StreamEndReasonNone:
		if info.IsStream {
			return OutcomeFailure
		}
	default:
		return OutcomeFailure
	}
	switch stream.Response {
	case relaycommon.ResponseOutcomeFailed, relaycommon.ResponseOutcomeCancelled:
		return OutcomeFailure
	case relaycommon.ResponseOutcomeIncomplete:
		if stream.IncompleteReason != "max_output_tokens" && stream.IncompleteReason != "max_tokens" {
			return OutcomeFailure
		}
	case relaycommon.ResponseOutcomeUnknown:
		if stream.ExpectsTerminal && stream.EndReason != relaycommon.StreamEndReasonDone {
			return OutcomeFailure
		}
	}
	return OutcomeSuccess
}
