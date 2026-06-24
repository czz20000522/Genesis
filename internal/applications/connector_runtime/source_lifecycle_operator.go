package connectorruntime

import (
	"context"
	"errors"
	"sort"
	"strings"
	"time"
)

func (s *FileSourceLifecycleStore) ClearBlockedSourceRun(ctx context.Context, sourceID string, reason string, now time.Time) (SourceRun, SourceOperatorActionRecord, error) {
	sourceID = strings.TrimSpace(sourceID)
	if sourceID == "" {
		return SourceRun{}, SourceOperatorActionRecord{}, errors.New("source id is required")
	}
	reason = defaultSourceOperatorReason(reason, "operator_cleared_blocked")
	if !safeConnectorCommandReason(reason) {
		return SourceRun{}, SourceOperatorActionRecord{}, errors.New("source operator reason is unsafe")
	}
	now = sourceOperatorTime(now)
	var run SourceRun
	var action SourceOperatorActionRecord
	err := s.withLockedState(ctx, func() error {
		existing, ok := s.runs[sourceID]
		if !ok {
			return errors.New("source run not found")
		}
		if existing.Status != SourceRunStatusBlocked {
			return errors.New("source run is not blocked")
		}
		previousStatus := existing.Status
		existing.Status = SourceRunStatusStopped
		existing.StoppedAt = now
		existing.BlockedReasonCode = ""
		existing.BlockedReason = ""
		existing.UpdatedAt = now
		run = existing
		action = SourceOperatorActionRecord{
			SourceID:       sourceID,
			Action:         SourceOperatorActionClearBlocked,
			Reason:         reason,
			PreviousStatus: previousStatus,
			NewStatus:      run.Status,
			CreatedAt:      now,
		}
		normalizedAction, err := normalizeSourceOperatorAction(action)
		if err != nil {
			return err
		}
		action = normalizedAction
		s.runs[sourceID] = run
		s.operatorActions[sourceID] = append(s.operatorActions[sourceID], action)
		sortSourceOperatorActions(s.operatorActions[sourceID])
		return s.writeLocked()
	})
	return run, action, err
}

func (s *FileSourceLifecycleStore) RequestSourceRestart(ctx context.Context, sourceID string, reason string, now time.Time) (SourceOperatorActionRecord, error) {
	sourceID = strings.TrimSpace(sourceID)
	if sourceID == "" {
		return SourceOperatorActionRecord{}, errors.New("source id is required")
	}
	reason = defaultSourceOperatorReason(reason, "operator_requested_restart")
	if !safeConnectorCommandReason(reason) {
		return SourceOperatorActionRecord{}, errors.New("source operator reason is unsafe")
	}
	now = sourceOperatorTime(now)
	var action SourceOperatorActionRecord
	err := s.withLockedState(ctx, func() error {
		run, ok := s.runs[sourceID]
		if !ok {
			return errors.New("source run not found")
		}
		action = SourceOperatorActionRecord{
			SourceID:       sourceID,
			Action:         SourceOperatorActionRequestRestart,
			Reason:         reason,
			PreviousStatus: run.Status,
			NewStatus:      run.Status,
			CreatedAt:      now,
		}
		normalizedAction, err := normalizeSourceOperatorAction(action)
		if err != nil {
			return err
		}
		action = normalizedAction
		s.operatorActions[sourceID] = append(s.operatorActions[sourceID], action)
		sortSourceOperatorActions(s.operatorActions[sourceID])
		return s.writeLocked()
	})
	return action, err
}

func (s *FileSourceLifecycleStore) ResetSourceCursor(ctx context.Context, sourceID string, cursorKind string, cursorValue string, reason string, acceptedDuplicateRisk bool, now time.Time) (SourceCursor, SourceOperatorActionRecord, error) {
	sourceID = strings.TrimSpace(sourceID)
	cursorKind = strings.TrimSpace(cursorKind)
	cursorValue = strings.TrimSpace(cursorValue)
	if sourceID == "" {
		return SourceCursor{}, SourceOperatorActionRecord{}, errors.New("source id is required")
	}
	if cursorKind == "" {
		cursorKind = SourceCursorKindExternalEventID
	}
	if !acceptedDuplicateRisk {
		return SourceCursor{}, SourceOperatorActionRecord{}, errors.New("source cursor reset requires accepted duplicate-processing risk")
	}
	reason = defaultSourceOperatorReason(reason, "operator_reset_cursor")
	if !safeConnectorCommandReason(reason) {
		return SourceCursor{}, SourceOperatorActionRecord{}, errors.New("source operator reason is unsafe")
	}
	now = sourceOperatorTime(now)
	var cursor SourceCursor
	var action SourceOperatorActionRecord
	err := s.withLockedState(ctx, func() error {
		if _, ok := s.runs[sourceID]; !ok {
			return errors.New("source run not found")
		}
		cursor = SourceCursor{
			SourceID:    sourceID,
			CursorKind:  cursorKind,
			CursorValue: cursorValue,
			UpdatedAt:   now,
		}
		normalizedCursor, err := normalizeSourceCursor(cursor)
		if err != nil {
			return err
		}
		cursor = normalizedCursor
		action = SourceOperatorActionRecord{
			SourceID:              sourceID,
			Action:                SourceOperatorActionResetCursor,
			Reason:                reason,
			CursorKind:            cursor.CursorKind,
			CursorValue:           cursor.CursorValue,
			AcceptedDuplicateRisk: true,
			CreatedAt:             now,
		}
		normalizedAction, err := normalizeSourceOperatorAction(action)
		if err != nil {
			return err
		}
		action = normalizedAction
		s.cursors[sourceCursorKey(cursor.SourceID, cursor.CursorKind)] = cursor
		s.operatorActions[sourceID] = append(s.operatorActions[sourceID], action)
		sortSourceOperatorActions(s.operatorActions[sourceID])
		return s.writeLocked()
	})
	return cursor, action, err
}

func (s *FileSourceLifecycleStore) ListSourceOperatorActions(ctx context.Context, sourceID string) ([]SourceOperatorActionRecord, error) {
	sourceID = strings.TrimSpace(sourceID)
	var actions []SourceOperatorActionRecord
	err := s.withLockedState(ctx, func() error {
		if sourceID == "" {
			for _, sourceActions := range s.operatorActions {
				actions = append(actions, sourceActions...)
			}
		} else {
			actions = append([]SourceOperatorActionRecord(nil), s.operatorActions[sourceID]...)
		}
		sortSourceOperatorActions(actions)
		return nil
	})
	return actions, err
}

func normalizeSourceOperatorAction(action SourceOperatorActionRecord) (SourceOperatorActionRecord, error) {
	action.ActionID = strings.TrimSpace(action.ActionID)
	action.SourceID = strings.TrimSpace(action.SourceID)
	action.Action = strings.TrimSpace(action.Action)
	action.Reason = strings.TrimSpace(action.Reason)
	action.PreviousStatus = strings.TrimSpace(action.PreviousStatus)
	action.NewStatus = strings.TrimSpace(action.NewStatus)
	action.CursorKind = strings.TrimSpace(action.CursorKind)
	action.CursorValue = strings.TrimSpace(action.CursorValue)
	switch {
	case action.SourceID == "":
		return SourceOperatorActionRecord{}, errors.New("source operator action source id is required")
	case !validSourceOperatorAction(action.Action):
		return SourceOperatorActionRecord{}, errors.New("source operator action is invalid")
	case !safeConnectorCommandReason(action.Reason):
		return SourceOperatorActionRecord{}, errors.New("source operator reason is unsafe")
	}
	if action.Action == SourceOperatorActionResetCursor {
		if action.CursorKind == "" || action.CursorValue == "" {
			return SourceOperatorActionRecord{}, errors.New("source cursor reset action requires cursor kind and value")
		}
		if !action.AcceptedDuplicateRisk {
			return SourceOperatorActionRecord{}, errors.New("source cursor reset action requires accepted duplicate risk")
		}
	}
	if action.CreatedAt.IsZero() {
		action.CreatedAt = time.Now().UTC()
	}
	if action.ActionID == "" {
		action.ActionID = stableOpaqueID(
			"source_operator",
			action.SourceID,
			action.Action,
			action.Reason,
			action.CursorKind,
			action.CursorValue,
			action.CreatedAt.Format(time.RFC3339Nano),
		)
	}
	return action, nil
}

func validSourceOperatorAction(action string) bool {
	switch action {
	case SourceOperatorActionClearBlocked, SourceOperatorActionRequestRestart, SourceOperatorActionResetCursor:
		return true
	default:
		return false
	}
}

func defaultSourceOperatorReason(reason string, fallback string) string {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return fallback
	}
	return reason
}

func sourceOperatorTime(now time.Time) time.Time {
	if now.IsZero() {
		return time.Now().UTC()
	}
	return now.UTC()
}

func sortSourceOperatorActions(actions []SourceOperatorActionRecord) {
	sort.Slice(actions, func(i, j int) bool {
		if !actions[i].CreatedAt.Equal(actions[j].CreatedAt) {
			return actions[i].CreatedAt.Before(actions[j].CreatedAt)
		}
		return actions[i].ActionID < actions[j].ActionID
	})
}
