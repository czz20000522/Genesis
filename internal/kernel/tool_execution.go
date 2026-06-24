package kernel

import (
	"context"
	"errors"
	"fmt"
)

type toolBatchExecutionOutcome struct {
	Completed bool
	Response  TurnResponse
}

func (k *Kernel) executeToolBatches(runCtx context.Context, toolGateway ToolGateway, sessionID string, turnID string, preparedCalls []preparedModelToolCall, toolCallEventIDs map[string]string) (toolBatchExecutionOutcome, error) {
	for _, batch := range planToolExecutionBatches(preparedCalls) {
		for _, callIndex := range batch.CallIndexes {
			call := preparedCalls[callIndex]
			result, err := toolGateway.Execute(runCtx, sessionID, turnID, call)
			if err != nil {
				if isTurnContextInterrupted(runCtx, err) {
					resp, completeErr := k.completeInterruptedTurn(sessionID, turnID)
					if completeErr != nil {
						return toolBatchExecutionOutcome{}, completeErr
					}
					return toolBatchExecutionOutcome{Completed: true, Response: resp}, nil
				}
				code := "tool_call_rejected"
				if errors.Is(err, ErrToolInfrastructureFailed) {
					code = "tool_infrastructure_failed"
				}
				failure := TurnError{
					Code:    code,
					Message: err.Error(),
				}
				if appendErr := k.appendTurnFailure(sessionID, turnID, failure); appendErr != nil {
					return toolBatchExecutionOutcome{}, appendErr
				}
				return toolBatchExecutionOutcome{}, err
			}
			forEventID := toolCallEventIDs[result.ToolCallEventID]
			if forEventID == "" {
				return toolBatchExecutionOutcome{}, fmt.Errorf("missing tool.call event for tool_call_event_id %q", result.ToolCallEventID)
			}
			if err := k.appendToolResultEvent(sessionID, turnID, result, forEventID); err != nil {
				return toolBatchExecutionOutcome{}, err
			}
			if result.PendingJobStart != nil {
				if err := k.startManagedJobExecutor(*result.PendingJobStart); err != nil {
					return toolBatchExecutionOutcome{}, err
				}
			}
			if isTurnContextInterrupted(runCtx, nil) {
				resp, completeErr := k.completeInterruptedTurn(sessionID, turnID)
				if completeErr != nil {
					return toolBatchExecutionOutcome{}, completeErr
				}
				return toolBatchExecutionOutcome{Completed: true, Response: resp}, nil
			}
		}
	}
	return toolBatchExecutionOutcome{}, nil
}
