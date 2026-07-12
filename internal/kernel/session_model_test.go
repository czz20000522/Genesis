package kernel

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
)

func TestBindSessionModelPersistsLatestBindingWithoutChangingAnotherSession(t *testing.T) {
	ledgerPath := filepath.Join(testTempDir(t), "events.sqlite")
	k := newSessionModelBindingKernel(t, ledgerPath)
	if err := k.BindSessionModel("session-a", SessionModelBindingRequest{ProfileID: "deepseek-flash"}); err != nil {
		t.Fatalf("BindSessionModel session a: %v", err)
	}
	if err := k.BindSessionModel("session-b", SessionModelBindingRequest{ProfileID: "local-qwen"}); err != nil {
		t.Fatalf("BindSessionModel session b: %v", err)
	}
	if err := k.BindSessionModel("session-a", SessionModelBindingRequest{ProfileID: "deepseek-pro"}); err != nil {
		t.Fatalf("BindSessionModel latest session a: %v", err)
	}
	events, err := k.loadEvents()
	if err != nil {
		t.Fatalf("loadEvents: %v", err)
	}
	for _, event := range events {
		if event.Type != "session.model_bound" || event.Data.SessionModel == nil || event.Data.SessionModel.ProfileID == "" {
			t.Fatalf("binding event = %+v, want only a profile id", event)
		}
	}
	k.Close()

	restarted := newSessionModelBindingKernel(t, ledgerPath)
	defer restarted.Close()
	for _, want := range []struct{ sessionID, profileID string }{
		{sessionID: "session-a", profileID: "deepseek-pro"},
		{sessionID: "session-b", profileID: "local-qwen"},
	} {
		projection, err := restarted.Session(want.sessionID)
		if err != nil {
			t.Fatalf("Session %s: %v", want.sessionID, err)
		}
		if projection.ModelProfileID != want.profileID {
			t.Fatalf("%s ModelProfileID = %q, want %q", want.sessionID, projection.ModelProfileID, want.profileID)
		}
	}
}

func TestBindSessionModelRejectsUnavailableProfileAndActiveSessionWithoutAppendingEvent(t *testing.T) {
	k := newSessionModelBindingKernel(t, filepath.Join(testTempDir(t), "events.sqlite"))
	defer k.Close()
	if err := k.BindSessionModel("session-a", SessionModelBindingRequest{ProfileID: "missing"}); !errors.Is(err, ErrSessionModelInvalid) {
		t.Fatalf("unavailable profile error = %v, want ErrSessionModelInvalid", err)
	}

	_, finish, admitted := k.tryBeginActiveTurn(nil, "session-a", "turn-a")
	if !admitted {
		t.Fatal("tryBeginActiveTurn did not admit test turn")
	}
	defer finish()
	if err := k.BindSessionModel("session-a", SessionModelBindingRequest{ProfileID: "deepseek-flash"}); !errors.Is(err, ErrSessionModelChangeBlockedActiveTurn) {
		t.Fatalf("active model bind error = %v, want ErrSessionModelChangeBlockedActiveTurn", err)
	}
	events, err := k.loadEvents()
	if err != nil {
		t.Fatalf("loadEvents: %v", err)
	}
	if len(events) != 0 {
		t.Fatalf("events = %+v, want no rejected binding fact", events)
	}
}

func TestBindSessionModelRejectsPausedTurnWithoutAppendingEvent(t *testing.T) {
	k := newSessionModelBindingKernel(t, filepath.Join(testTempDir(t), "events.sqlite"))
	defer k.Close()
	if err := k.BindSessionModel("session-a", SessionModelBindingRequest{ProfileID: "deepseek-flash"}); err != nil {
		t.Fatalf("initial binding: %v", err)
	}
	appendPausedSessionModelTurn(t, k, "session-a", "turn-paused")
	eventsBefore, err := k.loadEvents()
	if err != nil {
		t.Fatalf("load events before rejected binding: %v", err)
	}
	if err := k.BindSessionModel("session-a", SessionModelBindingRequest{ProfileID: "local-qwen"}); !errors.Is(err, ErrSessionModelChangeBlockedActiveTurn) {
		t.Fatalf("paused binding error = %v, want ErrSessionModelChangeBlockedActiveTurn", err)
	}
	eventsAfter, err := k.loadEvents()
	if err != nil {
		t.Fatalf("load events after rejected binding: %v", err)
	}
	if len(eventsAfter) != len(eventsBefore) {
		t.Fatalf("events after rejected paused binding = %d, want %d", len(eventsAfter), len(eventsBefore))
	}
}

func TestBindSessionModelReservesSessionBeforeResolverAndRejectsNilProvider(t *testing.T) {
	resolverEntered := make(chan struct{})
	releaseResolver := make(chan struct{})
	k, err := New(Config{
		LedgerPath:   filepath.Join(testTempDir(t), "events.sqlite"),
		RuntimeToken: testRuntimeToken,
		SessionProviderResolver: func(profileID string) (Provider, error) {
			switch profileID {
			case "blocking":
				close(resolverEntered)
				<-releaseResolver
				return FakeProvider{}, nil
			case "nil-provider":
				return nil, nil
			case "blocked":
				return NewBlockedProvider("blocked", "provider_credential_missing"), nil
			default:
				return nil, ErrSessionModelInvalid
			}
		},
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer k.Close()

	bindDone := make(chan error, 1)
	go func() {
		bindDone <- k.BindSessionModel("session-a", SessionModelBindingRequest{ProfileID: "blocking"})
	}()
	<-resolverEntered
	_, finish, admitted := k.tryBeginActiveTurn(nil, "session-a", "turn-during-binding")
	if admitted {
		finish()
		t.Fatal("turn admission succeeded while session model binding was in progress")
	}
	close(releaseResolver)
	if err := <-bindDone; err != nil {
		t.Fatalf("BindSessionModel blocking: %v", err)
	}
	if err := k.BindSessionModel("session-b", SessionModelBindingRequest{ProfileID: "nil-provider"}); !errors.Is(err, ErrSessionModelInvalid) {
		t.Fatalf("nil provider error = %v, want ErrSessionModelInvalid", err)
	}
	if err := k.BindSessionModel("session-c", SessionModelBindingRequest{ProfileID: "blocked"}); !errors.Is(err, ErrProviderUnavailable) {
		t.Fatalf("blocked provider error = %v, want ErrProviderUnavailable", err)
	}
	events, err := k.loadEvents()
	if err != nil {
		t.Fatalf("loadEvents: %v", err)
	}
	if len(events) != 1 || events[0].SessionID != "session-a" {
		t.Fatalf("events = %+v, want only accepted binding", events)
	}
}

func TestSessionModelBindingUsesBoundProviderWithoutGlobalCoordinatorFallback(t *testing.T) {
	deepseek := &namedSessionModelProvider{name: "deepseek", recorder: &recordingTextProvider{text: "deepseek reply"}}
	local := &namedSessionModelProvider{name: "local", recorder: &recordingTextProvider{text: "local reply"}}
	k, err := New(Config{
		LedgerPath:   filepath.Join(testTempDir(t), "events.sqlite"),
		Provider:     NewBlockedProvider("global", "global_coordinator_missing"),
		RuntimeToken: testRuntimeToken,
		SessionProviderResolver: func(profileID string) (Provider, error) {
			switch profileID {
			case "deepseek":
				return deepseek, nil
			case "local":
				return local, nil
			default:
				return nil, ErrSessionModelInvalid
			}
		},
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer k.Close()

	if _, err := k.SubmitTurn(nil, TurnRequest{SessionID: "a", InputItems: []InputItem{{Type: "text", Text: "unbound"}}}); !errors.Is(err, ErrSessionModelUnselected) {
		t.Fatalf("unbound SubmitTurn error = %v, want ErrSessionModelUnselected", err)
	}
	if err := k.BindSessionModel("a", SessionModelBindingRequest{ProfileID: "deepseek"}); err != nil {
		t.Fatalf("bind session a: %v", err)
	}
	first, err := k.SubmitTurn(nil, TurnRequest{SessionID: "a", InputItems: []InputItem{{Type: "text", Text: "first"}}})
	if err != nil {
		t.Fatalf("first session a turn: %v", err)
	}
	if first.Final.Text != "deepseek reply" {
		t.Fatalf("first final = %q, want deepseek reply", first.Final.Text)
	}
	if err := k.BindSessionModel("b", SessionModelBindingRequest{ProfileID: "local"}); err != nil {
		t.Fatalf("bind session b: %v", err)
	}
	if _, err := k.SubmitTurn(nil, TurnRequest{SessionID: "b", InputItems: []InputItem{{Type: "text", Text: "second"}}}); err != nil {
		t.Fatalf("session b turn: %v", err)
	}
	if err := k.BindSessionModel("a", SessionModelBindingRequest{ProfileID: "local"}); err != nil {
		t.Fatalf("switch session a: %v", err)
	}
	if _, err := k.SubmitTurn(nil, TurnRequest{SessionID: "a", InputItems: []InputItem{{Type: "text", Text: "third"}}}); err != nil {
		t.Fatalf("second session a turn: %v", err)
	}
	if len(deepseek.recorder.Requests()) != 1 || len(local.recorder.Requests()) != 2 {
		t.Fatalf("provider calls = deepseek:%d local:%d, want 1 and 2", len(deepseek.recorder.Requests()), len(local.recorder.Requests()))
	}
	context, err := k.ProviderContextProjection(first.TurnID)
	if err != nil {
		t.Fatalf("first ProviderContextProjection: %v", err)
	}
	if context.PrefixComponents.AdapterBinding != prefixDigest(providerPrefixIdentity(deepseek)) {
		t.Fatalf("first provider prefix binding = %q, want %q", context.PrefixComponents.AdapterBinding, prefixDigest(providerPrefixIdentity(deepseek)))
	}
}

func TestSessionProviderResolverKeepsKernelReadyWithoutGlobalCoordinator(t *testing.T) {
	k, err := New(Config{LedgerPath: filepath.Join(testTempDir(t), "events.sqlite"), Provider: NewBlockedProvider("global", "provider_profile_missing"), RuntimeToken: testRuntimeToken, SessionProviderResolver: func(string) (Provider, error) { return FakeProvider{}, nil }})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer k.Close()
	if ready := k.Ready(); ready.Readiness != ReadinessReady {
		t.Fatalf("Ready = %+v, want service ready", ready)
	}
}

func TestContinueDelegatedParentUsesOriginalTurnBindingAndFailsTerminallyWhenUnavailable(t *testing.T) {
	deepseek := &namedSessionModelProvider{name: "deepseek", recorder: &recordingTextProvider{text: "deepseek reply"}}
	blocked := false
	k, err := New(Config{
		LedgerPath:   filepath.Join(testTempDir(t), "events.sqlite"),
		RuntimeToken: testRuntimeToken,
		SessionProviderResolver: func(profileID string) (Provider, error) {
			if profileID != "deepseek" {
				return nil, ErrSessionModelInvalid
			}
			if blocked {
				return NewBlockedProvider("deepseek", "provider_credential_missing"), nil
			}
			return deepseek, nil
		},
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer k.Close()
	if err := k.BindSessionModel("session-a", SessionModelBindingRequest{ProfileID: "deepseek"}); err != nil {
		t.Fatalf("initial binding: %v", err)
	}
	appendPausedSessionModelTurn(t, k, "session-a", "turn-resume")
	k.continueDelegatedParent("session-a", "turn-resume")
	if len(deepseek.recorder.Requests()) != 1 {
		t.Fatalf("deepseek calls = %d, want 1", len(deepseek.recorder.Requests()))
	}
	if events, err := k.TurnEvents("turn-resume"); err != nil || !containsString(sessionModelEventTypes(events), "model.final") {
		t.Fatalf("resumed events = %+v err=%v, want model.final", events, err)
	}

	appendPausedSessionModelTurn(t, k, "session-a", "turn-unavailable")
	blocked = true
	k.continueDelegatedParent("session-a", "turn-unavailable")
	events, err := k.TurnEvents("turn-unavailable")
	if err != nil {
		t.Fatalf("unavailable resumed events: %v", err)
	}
	if !containsString(sessionModelEventTypes(events), "turn.failed") {
		t.Fatalf("unavailable resumed events = %v, want terminal turn.failed", sessionModelEventTypes(events))
	}
}

func appendPausedSessionModelTurn(t *testing.T, k *Kernel, sessionID string, turnID string) {
	t.Helper()
	now := k.clock()
	if err := k.appendEvent(StoredEvent{
		EventID:   newID("evt", now),
		SessionID: sessionID,
		TurnID:    turnID,
		Type:      "turn.submitted",
		CreatedAt: now,
		Data:      EventData{InputItems: []InputItem{{Type: "text", Text: "resume"}}},
	}); err != nil {
		t.Fatalf("append turn.submitted: %v", err)
	}
	if err := k.appendEvent(StoredEvent{
		EventID:   newID("evt", now),
		SessionID: sessionID,
		TurnID:    turnID,
		Type:      "turn.paused",
		CreatedAt: now,
	}); err != nil {
		t.Fatalf("append turn.paused: %v", err)
	}
}

func sessionModelEventTypes(events []Event) []string {
	types := make([]string, 0, len(events))
	for _, event := range events {
		types = append(types, event.Type)
	}
	return types
}

type namedSessionModelProvider struct {
	name     string
	recorder *recordingTextProvider
}

func (p *namedSessionModelProvider) Name() string {
	return p.name
}

func (p *namedSessionModelProvider) Ready() ProviderStatus {
	return ProviderStatus{Name: p.Name(), Readiness: ReadinessReady}
}

func (p *namedSessionModelProvider) Complete(ctx context.Context, request ModelRequest) (ModelResponse, error) {
	return p.recorder.Complete(ctx, request)
}

func newSessionModelBindingKernel(t *testing.T, ledgerPath string) *Kernel {
	t.Helper()
	profiles := map[string]Provider{
		"deepseek-flash": FakeProvider{},
		"deepseek-pro":   FakeProvider{},
		"local-qwen":     FakeProvider{},
	}
	k, err := New(Config{
		LedgerPath:   ledgerPath,
		RuntimeToken: testRuntimeToken,
		SessionProviderResolver: func(profileID string) (Provider, error) {
			provider, ok := profiles[profileID]
			if !ok {
				return nil, ErrSessionModelInvalid
			}
			return provider, nil
		},
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return k
}
