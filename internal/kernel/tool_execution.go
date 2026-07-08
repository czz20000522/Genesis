package kernel

import (
	"context"
	"errors"
	"fmt"
	"sync"
)

type toolBatchExecutionOutcome struct {
	Completed bool
	Response  TurnResponse
}

type toolCallExecutionResult struct {
	CallIndex int
	Result    ModelToolResult
}

type toolBatchRunner func(context.Context, ToolGateway, string, string, []preparedModelToolCall, ToolExecutionBatch) ([]toolCallExecutionResult, error)

func (k *Kernel) executeToolBatches(runCtx context.Context, toolGateway ToolGateway, sessionID string, turnID string, preparedCalls []preparedModelToolCall, toolCallEventIDs map[string]string) (toolBatchExecutionOutcome, error) {
	return k.executeToolBatchesWithRunner(runCtx, toolGateway, sessionID, turnID, preparedCalls, toolCallEventIDs, nil)
}

func (k *Kernel) executeToolBatchesWithRunner(runCtx context.Context, toolGateway ToolGateway, sessionID string, turnID string, preparedCalls []preparedModelToolCall, toolCallEventIDs map[string]string, runner toolBatchRunner) (toolBatchExecutionOutcome, error) {
	return k.executeToolBatchesWithGuard(runCtx, toolGateway, sessionID, turnID, preparedCalls, toolCallEventIDs, runner, nil)
}

func (k *Kernel) executeToolBatchesGuarded(runCtx context.Context, toolGateway ToolGateway, sessionID string, turnID string, preparedCalls []preparedModelToolCall, toolCallEventIDs map[string]string, guard *toolLoopGuard) (toolBatchExecutionOutcome, error) {
	return k.executeToolBatchesWithGuard(runCtx, toolGateway, sessionID, turnID, preparedCalls, toolCallEventIDs, nil, guard)
}

func (k *Kernel) executeToolBatchesWithGuard(runCtx context.Context, toolGateway ToolGateway, sessionID string, turnID string, preparedCalls []preparedModelToolCall, toolCallEventIDs map[string]string, runner toolBatchRunner, guard *toolLoopGuard) (toolBatchExecutionOutcome, error) {
	for _, batch := range planToolExecutionBatches(preparedCalls) {
		batchRunner := runner
		if batchRunner == nil && canExecuteToolBatchConcurrently(batch, preparedCalls) {
			batchRunner = executeToolBatchConcurrently
		}
		if batchRunner == nil {
			outcome, err := k.executeToolBatchSeriallyGuarded(runCtx, toolGateway, sessionID, turnID, preparedCalls, toolCallEventIDs, batch, guard)
			if err != nil || outcome.Completed {
				return outcome, err
			}
			continue
		}
		results, err := batchRunner(runCtx, toolGateway, sessionID, turnID, preparedCalls, batch)
		if err != nil {
			return k.handleToolExecutionError(runCtx, sessionID, turnID, err)
		}
		resultByCallIndex, err := indexToolExecutionResults(batch, results)
		if err != nil {
			return k.handleToolExecutionError(runCtx, sessionID, turnID, fmt.Errorf("%w: %v", ErrToolInfrastructureFailed, err))
		}
		for _, callIndex := range batch.CallIndexes {
			result, err := observeToolLoopGuardResult(guard, preparedCalls[callIndex], resultByCallIndex[callIndex])
			if err != nil {
				return toolBatchExecutionOutcome{}, err
			}
			outcome, err := k.commitToolExecutionResult(runCtx, sessionID, turnID, result, toolCallEventIDs)
			if err != nil {
				return toolBatchExecutionOutcome{}, err
			}
			if outcome.Completed {
				return outcome, nil
			}
		}
	}
	return toolBatchExecutionOutcome{}, nil
}

func canExecuteToolBatchConcurrently(batch ToolExecutionBatch, preparedCalls []preparedModelToolCall) bool {
	if !batch.Parallel || len(batch.CallIndexes) <= 1 || batch.Reason != ToolEffectClassPureRead {
		return false
	}
	for _, callIndex := range batch.CallIndexes {
		if callIndex < 0 || callIndex >= len(preparedCalls) {
			return false
		}
		if preparedCalls[callIndex].accessPlan.ParallelClass() != ToolEffectClassPureRead {
			return false
		}
	}
	return true
}

func executeToolBatchConcurrently(runCtx context.Context, toolGateway ToolGateway, sessionID string, turnID string, preparedCalls []preparedModelToolCall, batch ToolExecutionBatch) ([]toolCallExecutionResult, error) {
	ctx, cancel := context.WithCancel(runCtx)
	defer cancel()
	results := make([]toolCallExecutionResult, len(batch.CallIndexes))
	errCh := make(chan error, len(batch.CallIndexes))
	var wg sync.WaitGroup
	for resultIndex, callIndex := range batch.CallIndexes {
		resultIndex := resultIndex
		callIndex := callIndex
		wg.Add(1)
		go func() {
			defer wg.Done()
			result, err := toolGateway.Execute(ctx, sessionID, turnID, preparedCalls[callIndex])
			if err != nil {
				cancel()
				errCh <- err
				return
			}
			results[resultIndex] = toolCallExecutionResult{
				CallIndex: callIndex,
				Result:    result,
			}
		}()
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		if err != nil {
			return nil, err
		}
	}
	return results, nil
}

func guardToolLoopBeforeExecution(guard *toolLoopGuard, call preparedModelToolCall) (ModelToolResult, bool, error) {
	if guard == nil {
		return ModelToolResult{}, false, nil
	}
	return guard.beforeExecute(call)
}

func observeToolLoopGuardResult(guard *toolLoopGuard, call preparedModelToolCall, result ModelToolResult) (ModelToolResult, error) {
	if guard == nil {
		return result, nil
	}
	return guard.afterExecute(call, result)
}

func (k *Kernel) executeToolBatchSerially(runCtx context.Context, toolGateway ToolGateway, sessionID string, turnID string, preparedCalls []preparedModelToolCall, toolCallEventIDs map[string]string, batch ToolExecutionBatch) (toolBatchExecutionOutcome, error) {
	for _, callIndex := range batch.CallIndexes {
		result, err := toolGateway.Execute(runCtx, sessionID, turnID, preparedCalls[callIndex])
		if err != nil {
			return k.handleToolExecutionError(runCtx, sessionID, turnID, err)
		}
		outcome, err := k.commitToolExecutionResult(runCtx, sessionID, turnID, result, toolCallEventIDs)
		if err != nil || outcome.Completed {
			return outcome, err
		}
	}
	return toolBatchExecutionOutcome{}, nil
}

func (k *Kernel) executeToolBatchSeriallyGuarded(runCtx context.Context, toolGateway ToolGateway, sessionID string, turnID string, preparedCalls []preparedModelToolCall, toolCallEventIDs map[string]string, batch ToolExecutionBatch, guard *toolLoopGuard) (toolBatchExecutionOutcome, error) {
	for _, callIndex := range batch.CallIndexes {
		if result, blocked, err := guardToolLoopBeforeExecution(guard, preparedCalls[callIndex]); err != nil || blocked {
			if err != nil {
				return toolBatchExecutionOutcome{}, err
			}
			outcome, err := k.commitToolExecutionResult(runCtx, sessionID, turnID, result, toolCallEventIDs)
			if err != nil || outcome.Completed {
				return outcome, err
			}
			continue
		}
		result, err := toolGateway.Execute(runCtx, sessionID, turnID, preparedCalls[callIndex])
		if err != nil {
			return k.handleToolExecutionError(runCtx, sessionID, turnID, err)
		}
		result, err = observeToolLoopGuardResult(guard, preparedCalls[callIndex], result)
		if err != nil {
			return toolBatchExecutionOutcome{}, err
		}
		outcome, err := k.commitToolExecutionResult(runCtx, sessionID, turnID, result, toolCallEventIDs)
		if err != nil || outcome.Completed {
			return outcome, err
		}
	}
	return toolBatchExecutionOutcome{}, nil
}

func indexToolExecutionResults(batch ToolExecutionBatch, results []toolCallExecutionResult) (map[int]ModelToolResult, error) {
	expected := make(map[int]struct{}, len(batch.CallIndexes))
	for _, callIndex := range batch.CallIndexes {
		expected[callIndex] = struct{}{}
	}
	indexed := make(map[int]ModelToolResult, len(results))
	for _, item := range results {
		if _, ok := expected[item.CallIndex]; !ok {
			return nil, fmt.Errorf("tool batch runner returned result for unexpected call index %d", item.CallIndex)
		}
		if _, exists := indexed[item.CallIndex]; exists {
			return nil, fmt.Errorf("tool batch runner returned duplicate result for call index %d", item.CallIndex)
		}
		indexed[item.CallIndex] = item.Result
	}
	for _, callIndex := range batch.CallIndexes {
		if _, ok := indexed[callIndex]; !ok {
			return nil, fmt.Errorf("tool batch runner returned no result for call index %d", callIndex)
		}
	}
	return indexed, nil
}

func (k *Kernel) handleToolExecutionError(runCtx context.Context, sessionID string, turnID string, err error) (toolBatchExecutionOutcome, error) {
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
		Message: externalBoundaryDiagnosticText(err.Error()),
	}
	if appendErr := k.appendTurnFailure(sessionID, turnID, failure); appendErr != nil {
		return toolBatchExecutionOutcome{}, appendErr
	}
	return toolBatchExecutionOutcome{}, err
}

func (k *Kernel) commitToolExecutionResult(runCtx context.Context, sessionID string, turnID string, result ModelToolResult, toolCallEventIDs map[string]string) (toolBatchExecutionOutcome, error) {
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
	return toolBatchExecutionOutcome{}, nil
}
