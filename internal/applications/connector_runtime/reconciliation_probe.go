package connectorruntime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"
)

type ConnectorReconciliationProbe interface {
	Probe(context.Context, ConnectorOutboxItem, ReconciliationLookup) (ReconciliationProbeResult, error)
}

type ReconciliationCommandAdapter struct {
	Executable string
	Args       []string
	Runner     CommandRunner
}

func (a ReconciliationCommandAdapter) Probe(ctx context.Context, item ConnectorOutboxItem, lookup ReconciliationLookup) (ReconciliationProbeResult, error) {
	if err := validateReconciliationLookup(lookup); err != nil {
		return ReconciliationProbeResult{ObservedStatus: ReconciliationObservedUnavailable, Reason: ReconciliationReasonMissingHandle}, err
	}
	request := ReconciliationProbeRequest{
		OutboxID:       item.OutboxID,
		Connector:      item.Connector,
		ActionKind:     item.ActionKind,
		IdempotencyKey: item.IdempotencyKey,
		Lookup:         lookup,
	}
	payload, err := json.Marshal(request)
	if err != nil {
		return ReconciliationProbeResult{ObservedStatus: ReconciliationObservedUnavailable, Reason: ReconciliationReasonExternalUnavailable}, err
	}
	runner := a.Runner
	if runner == nil {
		runner = OSCommandRunner{}
	}
	output, err := runner.Run(ctx, a.Executable, append(a.Args, string(payload))...)
	if err != nil {
		return ReconciliationProbeResult{ObservedStatus: ReconciliationObservedUnavailable, Reason: ReconciliationReasonExternalUnavailable}, err
	}
	result, err := decodeReconciliationProbeResult(string(output))
	if err != nil {
		return ReconciliationProbeResult{ObservedStatus: ReconciliationObservedUnavailable, Reason: ReconciliationReasonExternalUnavailable}, err
	}
	if err := validateReconciliationProbeResult(result); err != nil {
		return ReconciliationProbeResult{ObservedStatus: ReconciliationObservedUnavailable, Reason: ReconciliationReasonExternalUnavailable}, err
	}
	return result, nil
}

func (r *Runtime) ProbeRecoveryRequiredOutboxItem(ctx context.Context, outboxID string, lookup ReconciliationLookup) (ReconciliationEvidence, error) {
	if r.Store == nil {
		return ReconciliationEvidence{}, errors.New("connector runtime missing outbox store")
	}
	now := time.Now()
	if r.Now != nil {
		now = r.Now()
	}
	item, err := r.Store.GetOutboxItem(ctx, strings.TrimSpace(outboxID))
	if err != nil {
		return ReconciliationEvidence{}, err
	}
	if item.Status != OutboxStatusRecoveryRequired {
		return ReconciliationEvidence{}, errors.New("outbox item is not recovery-required")
	}
	if err := validateReconciliationLookup(lookup); err != nil {
		evidence := r.reconciliationEvidenceFor(item, lookup, ReconciliationProbeResult{
			ObservedStatus: ReconciliationObservedUnavailable,
			Reason:         ReconciliationReasonMissingHandle,
		}, now)
		if recordErr := r.Store.RecordReconciliationEvidence(ctx, evidence); recordErr != nil {
			return evidence, recordErr
		}
		return evidence, err
	}
	probe := r.ReconciliationProbes[item.Connector]
	if probe == nil {
		evidence := r.reconciliationEvidenceFor(item, lookup, ReconciliationProbeResult{
			ObservedStatus: ReconciliationObservedUnavailable,
			Reason:         ReconciliationReasonUnsupportedAction,
		}, now)
		if recordErr := r.Store.RecordReconciliationEvidence(ctx, evidence); recordErr != nil {
			return evidence, recordErr
		}
		return evidence, errors.New("connector reconciliation probe unsupported")
	}
	result, probeErr := probe.Probe(ctx, item, lookup)
	if result.ObservedStatus == "" {
		result.ObservedStatus = ReconciliationObservedUnavailable
	}
	if result.Reason == "" && probeErr != nil {
		result.Reason = ReconciliationReasonExternalUnavailable
	}
	if err := validateReconciliationProbeResult(result); err != nil {
		result = ReconciliationProbeResult{
			ObservedStatus: ReconciliationObservedUnavailable,
			Reason:         ReconciliationReasonExternalUnavailable,
		}
		if probeErr == nil {
			probeErr = err
		}
	}
	evidence := r.reconciliationEvidenceFor(item, lookup, result, now)
	if recordErr := r.Store.RecordReconciliationEvidence(ctx, evidence); recordErr != nil {
		return evidence, recordErr
	}
	return evidence, probeErr
}

func (r *Runtime) reconciliationEvidenceFor(item ConnectorOutboxItem, lookup ReconciliationLookup, result ReconciliationProbeResult, now time.Time) ReconciliationEvidence {
	return ReconciliationEvidence{
		ProbeID:           stableOpaqueID("recon", item.OutboxID, lookup.Kind, lookup.Value, result.ObservedStatus, result.Reason, now.Format(time.RFC3339Nano)),
		OutboxID:          item.OutboxID,
		Connector:         item.Connector,
		ActionKind:        item.ActionKind,
		LookupKind:        strings.TrimSpace(lookup.Kind),
		LookupValue:       strings.TrimSpace(lookup.Value),
		ObservedStatus:    strings.TrimSpace(result.ObservedStatus),
		Reason:            strings.TrimSpace(result.Reason),
		ExternalActionRef: strings.TrimSpace(result.ExternalActionRef),
		CheckedAt:         now,
	}
}

func normalizeReconciliationEvidence(evidence ReconciliationEvidence) (ReconciliationEvidence, error) {
	evidence.OutboxID = strings.TrimSpace(evidence.OutboxID)
	evidence.Connector = strings.TrimSpace(evidence.Connector)
	evidence.ActionKind = strings.TrimSpace(evidence.ActionKind)
	evidence.LookupKind = strings.TrimSpace(evidence.LookupKind)
	evidence.LookupValue = strings.TrimSpace(evidence.LookupValue)
	evidence.ObservedStatus = strings.TrimSpace(evidence.ObservedStatus)
	evidence.Reason = strings.TrimSpace(evidence.Reason)
	evidence.ExternalActionRef = strings.TrimSpace(evidence.ExternalActionRef)
	if evidence.OutboxID == "" || evidence.Connector == "" || evidence.ActionKind == "" {
		return ReconciliationEvidence{}, errors.New("reconciliation evidence missing outbox identity")
	}
	if evidence.ProbeID == "" {
		evidence.ProbeID = stableOpaqueID("recon", evidence.OutboxID, evidence.LookupKind, evidence.LookupValue, evidence.ObservedStatus, evidence.Reason, evidence.CheckedAt.Format(time.RFC3339Nano))
	}
	if evidence.CheckedAt.IsZero() {
		evidence.CheckedAt = time.Now()
	}
	if err := validateReconciliationObservedStatus(evidence.ObservedStatus); err != nil {
		return ReconciliationEvidence{}, err
	}
	if evidence.ExternalActionRef != "" && !safeExternalActionRef(evidence.ExternalActionRef) {
		return ReconciliationEvidence{}, errors.New("reconciliation evidence external action ref is unsafe")
	}
	if !safeConnectorCommandReason(evidence.Reason) {
		return ReconciliationEvidence{}, errors.New("reconciliation evidence reason is unsafe")
	}
	return evidence, nil
}

func validateReconciliationLookup(lookup ReconciliationLookup) error {
	kind := strings.TrimSpace(lookup.Kind)
	value := strings.TrimSpace(lookup.Value)
	if kind == "" || value == "" {
		return errors.New("reconciliation probe requires exact lookup handle")
	}
	switch kind {
	case ReconciliationLookupActionID,
		ReconciliationLookupIdempotencyKey,
		ReconciliationLookupExternalReceiptRef,
		ReconciliationLookupExternalActionRef,
		ReconciliationLookupAdapterExactRef:
	default:
		return fmt.Errorf("reconciliation probe lookup kind %q is not an exact supported handle", kind)
	}
	if isCredentialShapedExternalValue(value) || strings.ContainsAny(value, "\r\n\t") {
		return errors.New("reconciliation probe lookup value is unsafe")
	}
	return nil
}

func decodeReconciliationProbeResult(text string) (ReconciliationProbeResult, error) {
	if strings.TrimSpace(text) == "" {
		return ReconciliationProbeResult{}, errors.New("reconciliation probe returned empty result")
	}
	decoder := json.NewDecoder(strings.NewReader(text))
	decoder.DisallowUnknownFields()
	var result ReconciliationProbeResult
	if err := decoder.Decode(&result); err != nil {
		return ReconciliationProbeResult{}, fmt.Errorf("decode reconciliation probe result: %w", err)
	}
	var trailing struct{}
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return ReconciliationProbeResult{}, errors.New("reconciliation probe returned multiple JSON values")
	}
	return result, nil
}

func validateReconciliationProbeResult(result ReconciliationProbeResult) error {
	if err := validateReconciliationObservedStatus(strings.TrimSpace(result.ObservedStatus)); err != nil {
		return err
	}
	if result.ExternalActionRef != "" && !safeExternalActionRef(result.ExternalActionRef) {
		return errors.New("reconciliation probe returned unsafe external action ref")
	}
	if !safeConnectorCommandReason(result.Reason) {
		return errors.New("reconciliation probe returned unsafe reason")
	}
	return nil
}

func validateReconciliationObservedStatus(status string) error {
	switch status {
	case ReconciliationObservedSent,
		ReconciliationObservedNotFound,
		ReconciliationObservedFailed,
		ReconciliationObservedUnknown,
		ReconciliationObservedUnavailable,
		ReconciliationObservedAmbiguous,
		ReconciliationObservedPermissionDenied:
		return nil
	default:
		return fmt.Errorf("unsupported reconciliation observed status %q", status)
	}
}
