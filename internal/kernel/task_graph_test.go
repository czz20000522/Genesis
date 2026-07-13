package kernel

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func TestTaskGraphDependenciesProjectReadyAndSurviveRestart(t *testing.T) {
	ledgerPath := filepath.Join(testTempDir(t), "events.sqlite")
	k := newTestKernel(t, ledgerPath)
	first, err := k.AdmitAgentInvocation(AgentInvocationAdmissionRequest{SessionID: "graph-session", Principal: "application:test", CapabilityGrant: CapabilityGrant{ToolNames: []string{"resource_read"}}})
	if err != nil {
		t.Fatalf("AdmitAgentInvocation first: %v", err)
	}
	second, err := k.AdmitAgentInvocation(AgentInvocationAdmissionRequest{SessionID: "graph-session", Principal: "application:test", CapabilityGrant: CapabilityGrant{ToolNames: []string{"resource_read"}}})
	if err != nil {
		t.Fatalf("AdmitAgentInvocation second: %v", err)
	}
	graph, err := k.CreateTaskGraph(TaskGraphCreateRequest{SessionID: "graph-session"})
	if err != nil {
		t.Fatalf("CreateTaskGraph: %v", err)
	}
	firstNode, err := k.AddTaskGraphNode(TaskGraphNodeRequest{GraphID: graph.GraphID})
	if err != nil {
		t.Fatalf("AddTaskGraphNode first: %v", err)
	}
	secondNode, err := k.AddTaskGraphNode(TaskGraphNodeRequest{GraphID: graph.GraphID})
	if err != nil {
		t.Fatalf("AddTaskGraphNode second: %v", err)
	}
	if err := k.AddTaskGraphEdge(TaskGraphEdgeRequest{GraphID: graph.GraphID, FromNodeID: firstNode.NodeID, ToNodeID: secondNode.NodeID}); err != nil {
		t.Fatalf("AddTaskGraphEdge: %v", err)
	}
	if _, err := k.BindTaskGraphNodeInvocation(TaskGraphNodeBindingRequest{GraphID: graph.GraphID, NodeID: firstNode.NodeID, InvocationID: first.InvocationID}); err != nil {
		t.Fatalf("bind first node: %v", err)
	}
	if err := k.TransitionTaskGraphNode(TaskGraphNodeTransitionRequest{GraphID: graph.GraphID, NodeID: firstNode.NodeID, Status: TaskGraphNodeStatusCompleted}); err != nil {
		t.Fatalf("complete first node: %v", err)
	}
	if _, err := k.BindTaskGraphNodeInvocation(TaskGraphNodeBindingRequest{GraphID: graph.GraphID, NodeID: secondNode.NodeID, InvocationID: second.InvocationID}); err != nil {
		t.Fatalf("bind second node: %v", err)
	}
	projection, err := k.TaskGraph(graph.GraphID)
	if err != nil {
		t.Fatalf("TaskGraph: %v", err)
	}
	if taskGraphNodeByID(t, projection, secondNode.NodeID).Status != TaskGraphNodeStatusReady {
		t.Fatalf("dependent node = %+v, want ready", taskGraphNodeByID(t, projection, secondNode.NodeID))
	}
	k.Close()
	restarted := newTestKernel(t, ledgerPath)
	projection, err = restarted.TaskGraph(graph.GraphID)
	if err != nil || len(projection.Nodes) != 2 || taskGraphNodeByID(t, projection, secondNode.NodeID).Status != TaskGraphNodeStatusReady {
		t.Fatalf("restarted graph = %+v error = %v, want ready dependent", projection, err)
	}
}

func TestTaskGraphPersistsUnboundTaskMetadata(t *testing.T) {
	k := newTestKernel(t, filepath.Join(testTempDir(t), "events.sqlite"))
	graph, err := k.CreateTaskGraph(TaskGraphCreateRequest{SessionID: "graph-metadata"})
	if err != nil {
		t.Fatalf("create graph: %v", err)
	}
	node, err := k.AddTaskGraphNode(TaskGraphNodeRequest{GraphID: graph.GraphID, Title: "确认发布条件", Description: "等待用户完成最终确认"})
	if err != nil {
		t.Fatalf("add generic node: %v", err)
	}
	projection, err := k.TaskGraph(graph.GraphID)
	if err != nil {
		t.Fatalf("read graph: %v", err)
	}
	stored := taskGraphNodeByID(t, projection, node.NodeID)
	if stored.Title != "确认发布条件" || stored.Description != "等待用户完成最终确认" || stored.InvocationID != "" {
		t.Fatalf("generic node = %+v", stored)
	}
}

func TestTaskGraphBindsReadyInvocationWithoutDispatch(t *testing.T) {
	k := newTestKernel(t, filepath.Join(testTempDir(t), "events.sqlite"))
	graph, err := k.CreateTaskGraph(TaskGraphCreateRequest{SessionID: "graph-bind"})
	if err != nil {
		t.Fatalf("create graph: %v", err)
	}
	node, err := k.AddTaskGraphNode(TaskGraphNodeRequest{GraphID: graph.GraphID, Title: "inspect repository"})
	if err != nil {
		t.Fatalf("add node: %v", err)
	}
	invocation, err := k.AdmitAgentInvocation(AgentInvocationAdmissionRequest{SessionID: "graph-bind", Principal: "application:test", CapabilityGrant: CapabilityGrant{ToolNames: []string{"resource_read"}}})
	if err != nil {
		t.Fatalf("admit invocation: %v", err)
	}
	if _, err := k.BindTaskGraphNodeInvocation(TaskGraphNodeBindingRequest{GraphID: graph.GraphID, NodeID: node.NodeID, InvocationID: invocation.InvocationID}); err != nil {
		t.Fatalf("bind ready node: %v", err)
	}
	projection, err := k.TaskGraph(graph.GraphID)
	if err != nil {
		t.Fatalf("read graph: %v", err)
	}
	bound := taskGraphNodeByID(t, projection, node.NodeID)
	if bound.InvocationID != invocation.InvocationID || bound.Status != TaskGraphNodeStatusReady {
		t.Fatalf("bound node = %+v, want ready node with existing invocation", bound)
	}
	events, err := k.loadEvents()
	if err != nil {
		t.Fatalf("load events: %v", err)
	}
	if countEvents(events, "agent_invocation.run_started") != 0 {
		t.Fatalf("binding dispatched invocation: %+v", events)
	}
}

func TestTaskGraphRejectsDuplicateInvocationBinding(t *testing.T) {
	k := newTestKernel(t, filepath.Join(testTempDir(t), "events.sqlite"))
	graph, err := k.CreateTaskGraph(TaskGraphCreateRequest{SessionID: "graph-one-invocation"})
	if err != nil {
		t.Fatalf("create graph: %v", err)
	}
	first, err := k.AddTaskGraphNode(TaskGraphNodeRequest{GraphID: graph.GraphID})
	if err != nil {
		t.Fatalf("add first: %v", err)
	}
	second, err := k.AddTaskGraphNode(TaskGraphNodeRequest{GraphID: graph.GraphID})
	if err != nil {
		t.Fatalf("add second: %v", err)
	}
	invocation, err := k.AdmitAgentInvocation(AgentInvocationAdmissionRequest{SessionID: graph.SessionID, Principal: "application:test", CapabilityGrant: CapabilityGrant{}})
	if err != nil {
		t.Fatalf("admit invocation: %v", err)
	}
	if _, err := k.BindTaskGraphNodeInvocation(TaskGraphNodeBindingRequest{GraphID: graph.GraphID, NodeID: first.NodeID, InvocationID: invocation.InvocationID}); err != nil {
		t.Fatalf("bind first: %v", err)
	}
	before, err := k.loadEvents()
	if err != nil {
		t.Fatalf("load before: %v", err)
	}
	if _, err := k.BindTaskGraphNodeInvocation(TaskGraphNodeBindingRequest{GraphID: graph.GraphID, NodeID: second.NodeID, InvocationID: invocation.InvocationID}); err == nil {
		t.Fatal("duplicate invocation binding accepted")
	}
	after, err := k.loadEvents()
	if err != nil || len(after) != len(before) {
		t.Fatalf("duplicate binding appended event %d -> %d error %v", len(before), len(after), err)
	}
}

func TestTaskGraphReducesRunningInvocationBeforeTopologyCanChange(t *testing.T) {
	k := newTestKernel(t, filepath.Join(testTempDir(t), "events.sqlite"))
	graph, err := k.CreateTaskGraph(TaskGraphCreateRequest{SessionID: "graph-running"})
	if err != nil {
		t.Fatalf("create graph: %v", err)
	}
	node, err := k.AddTaskGraphNode(TaskGraphNodeRequest{GraphID: graph.GraphID, Title: "inspect"})
	if err != nil {
		t.Fatalf("add node: %v", err)
	}
	invocation, err := k.AdmitAgentInvocation(AgentInvocationAdmissionRequest{SessionID: graph.SessionID, Principal: "application:test", CapabilityGrant: CapabilityGrant{}})
	if err != nil {
		t.Fatalf("admit invocation: %v", err)
	}
	if _, err := k.BindTaskGraphNodeInvocation(TaskGraphNodeBindingRequest{GraphID: graph.GraphID, NodeID: node.NodeID, InvocationID: invocation.InvocationID}); err != nil {
		t.Fatalf("bind node: %v", err)
	}
	run := AgentInvocationRunProjection{InvocationID: invocation.InvocationID, RunID: "run-running", SessionID: graph.SessionID, Principal: "application:test", Status: AgentInvocationRunStatusRunning}
	if err := k.appendAgentInvocationRunEvent("agent_invocation.run_started", run); err != nil {
		t.Fatalf("append running run: %v", err)
	}
	projection, err := k.TaskGraph(graph.GraphID)
	if err != nil || taskGraphNodeByID(t, projection, node.NodeID).Status != TaskGraphNodeStatusRunning {
		t.Fatalf("running projection = %+v error = %v", projection, err)
	}
	before, err := k.loadEvents()
	if err != nil {
		t.Fatalf("load before: %v", err)
	}
	if err := k.UpdateTaskGraphNode(TaskGraphNodeUpdateRequest{GraphID: graph.GraphID, NodeID: node.NodeID, Title: "rewritten"}); err == nil {
		t.Fatal("running node metadata update accepted")
	}
	after, err := k.loadEvents()
	if err != nil || len(after) != len(before) {
		t.Fatalf("running mutation appended event %d -> %d error %v", len(before), len(after), err)
	}
}

func TestTaskGraphRejectsPreboundInvocationDuringTopologyProposal(t *testing.T) {
	k := newTestKernel(t, filepath.Join(testTempDir(t), "events.sqlite"))
	graph, err := k.CreateTaskGraph(TaskGraphCreateRequest{SessionID: "graph-prebound"})
	if err != nil {
		t.Fatalf("create graph: %v", err)
	}
	invocation, err := k.AdmitAgentInvocation(AgentInvocationAdmissionRequest{SessionID: "graph-prebound", Principal: "application:test", CapabilityGrant: CapabilityGrant{ToolNames: []string{"resource_read"}}})
	if err != nil {
		t.Fatalf("admit invocation: %v", err)
	}
	before, err := k.loadEvents()
	if err != nil {
		t.Fatalf("load before: %v", err)
	}
	if _, err := k.AddTaskGraphNode(TaskGraphNodeRequest{GraphID: graph.GraphID, InvocationID: invocation.InvocationID}); err == nil {
		t.Fatal("topology proposal accepted a prebound invocation")
	}
	after, err := k.loadEvents()
	if err != nil || len(after) != len(before) {
		t.Fatalf("prebound proposal appended fact %d -> %d error %v", len(before), len(after), err)
	}
}

func TestTaskGraphReducesBoundInvocationTerminalEvidenceAcrossRestart(t *testing.T) {
	ledgerPath := filepath.Join(testTempDir(t), "events.sqlite")
	k := newAgentInvocationRunTestKernel(t, Config{
		LedgerPath: ledgerPath,
		Provider:   &countingTextProvider{text: "worker complete"},
		ToolPolicy: ToolPolicy{PermissionMode: PermissionModePlan},
	})
	graph, err := k.CreateTaskGraph(TaskGraphCreateRequest{SessionID: "graph-reduce"})
	if err != nil {
		t.Fatalf("create graph: %v", err)
	}
	node, err := k.AddTaskGraphNode(TaskGraphNodeRequest{GraphID: graph.GraphID, Title: "inspect repository"})
	if err != nil {
		t.Fatalf("add node: %v", err)
	}
	invocation, err := k.AdmitAgentInvocation(AgentInvocationAdmissionRequest{SessionID: "graph-reduce", Principal: "application:test", CapabilityGrant: CapabilityGrant{}})
	if err != nil {
		t.Fatalf("admit invocation: %v", err)
	}
	if _, err := k.BindTaskGraphNodeInvocation(TaskGraphNodeBindingRequest{GraphID: graph.GraphID, NodeID: node.NodeID, InvocationID: invocation.InvocationID}); err != nil {
		t.Fatalf("bind node: %v", err)
	}
	if _, err := k.RunAgentInvocation(context.Background(), AgentInvocationRunRequest{InvocationID: invocation.InvocationID, Principal: "application:test", InputItems: []InputItem{{Type: "text", Text: "inspect"}}}); err != nil {
		t.Fatalf("run invocation: %v", err)
	}
	projection, err := k.TaskGraph(graph.GraphID)
	if err != nil {
		t.Fatalf("read graph: %v", err)
	}
	bound := taskGraphNodeByID(t, projection, node.NodeID)
	if bound.Status != TaskGraphNodeStatusCompleted || len(bound.EvidenceRefs) != 1 {
		t.Fatalf("terminal reduction = %+v, want completed node with one evidence ref", bound)
	}
	k.Close()
	restarted := newAgentInvocationRunTestKernel(t, Config{LedgerPath: ledgerPath, Provider: &countingTextProvider{text: "must not run"}, ToolPolicy: ToolPolicy{PermissionMode: PermissionModePlan}})
	projection, err = restarted.TaskGraph(graph.GraphID)
	if err != nil {
		t.Fatalf("read restarted graph: %v", err)
	}
	if bound = taskGraphNodeByID(t, projection, node.NodeID); bound.Status != TaskGraphNodeStatusCompleted || len(bound.EvidenceRefs) != 1 {
		t.Fatalf("restarted reduction = %+v, want completed node with one evidence ref", bound)
	}
}

func TestTaskGraphReconcilesTerminalInvocationAfterReductionAppendFailure(t *testing.T) {
	ledgerPath := filepath.Join(testTempDir(t), "events.sqlite")
	k := newAgentInvocationRunTestKernel(t, Config{LedgerPath: ledgerPath, Provider: &countingTextProvider{text: "worker complete"}, ToolPolicy: ToolPolicy{PermissionMode: PermissionModePlan}})
	graph, err := k.CreateTaskGraph(TaskGraphCreateRequest{SessionID: "graph-reconcile"})
	if err != nil {
		t.Fatalf("create graph: %v", err)
	}
	node, err := k.AddTaskGraphNode(TaskGraphNodeRequest{GraphID: graph.GraphID})
	if err != nil {
		t.Fatalf("add node: %v", err)
	}
	invocation, err := k.AdmitAgentInvocation(AgentInvocationAdmissionRequest{SessionID: graph.SessionID, Principal: "application:test", CapabilityGrant: CapabilityGrant{}})
	if err != nil {
		t.Fatalf("admit invocation: %v", err)
	}
	if _, err := k.BindTaskGraphNodeInvocation(TaskGraphNodeBindingRequest{GraphID: graph.GraphID, NodeID: node.NodeID, InvocationID: invocation.InvocationID}); err != nil {
		t.Fatalf("bind node: %v", err)
	}
	k.ledger = &failOnceTaskGraphTerminalReductionLedger{Ledger: k.ledger}
	if _, err := k.RunAgentInvocation(context.Background(), AgentInvocationRunRequest{InvocationID: invocation.InvocationID, Principal: "application:test", InputItems: []InputItem{{Type: "text", Text: "inspect"}}}); err == nil {
		t.Fatal("terminal graph-reduction append failure was hidden")
	}
	k.Close()
	restarted := newAgentInvocationRunTestKernel(t, Config{LedgerPath: ledgerPath, Provider: &countingTextProvider{text: "must not run"}, ToolPolicy: ToolPolicy{PermissionMode: PermissionModePlan}})
	projection, err := restarted.TaskGraph(graph.GraphID)
	if err != nil {
		t.Fatalf("read reconciled graph: %v", err)
	}
	bound := taskGraphNodeByID(t, projection, node.NodeID)
	if bound.Status != TaskGraphNodeStatusCompleted || len(bound.EvidenceRefs) != 1 {
		t.Fatalf("reconciled node = %+v, want completed terminal evidence", bound)
	}
}

func TestTaskGraphFailedInvocationBlocksDependentWithoutDispatch(t *testing.T) {
	k := newTestKernel(t, filepath.Join(testTempDir(t), "events.sqlite"))
	graph, err := k.CreateTaskGraph(TaskGraphCreateRequest{SessionID: "graph-failed-reduce"})
	if err != nil {
		t.Fatalf("create graph: %v", err)
	}
	first, err := k.AddTaskGraphNode(TaskGraphNodeRequest{GraphID: graph.GraphID, Title: "first"})
	if err != nil {
		t.Fatalf("add first: %v", err)
	}
	second, err := k.AddTaskGraphNode(TaskGraphNodeRequest{GraphID: graph.GraphID, Title: "second"})
	if err != nil {
		t.Fatalf("add second: %v", err)
	}
	if err := k.AddTaskGraphEdge(TaskGraphEdgeRequest{GraphID: graph.GraphID, FromNodeID: first.NodeID, ToNodeID: second.NodeID}); err != nil {
		t.Fatalf("add dependency: %v", err)
	}
	invocation, err := k.AdmitAgentInvocation(AgentInvocationAdmissionRequest{SessionID: graph.SessionID, Principal: "application:test", CapabilityGrant: CapabilityGrant{}})
	if err != nil {
		t.Fatalf("admit invocation: %v", err)
	}
	if _, err := k.BindTaskGraphNodeInvocation(TaskGraphNodeBindingRequest{GraphID: graph.GraphID, NodeID: first.NodeID, InvocationID: invocation.InvocationID}); err != nil {
		t.Fatalf("bind first: %v", err)
	}
	failed := AgentInvocationRunProjection{InvocationID: invocation.InvocationID, RunID: "run-failed", SessionID: graph.SessionID, Principal: "application:test", Status: AgentInvocationRunStatusFailed, Error: &TurnError{Code: "worker_failed", Message: "worker failed"}}
	if err := k.appendAgentInvocationRunEvent("agent_invocation.run_failed", failed); err != nil {
		t.Fatalf("append failed run: %v", err)
	}
	projection, err := k.TaskGraph(graph.GraphID)
	if err != nil {
		t.Fatalf("read graph: %v", err)
	}
	if node := taskGraphNodeByID(t, projection, first.NodeID); node.Status != TaskGraphNodeStatusFailed || len(node.EvidenceRefs) != 1 {
		t.Fatalf("failed node = %+v, want terminal evidence", node)
	}
	if node := taskGraphNodeByID(t, projection, second.NodeID); node.Status != TaskGraphNodeStatusBlocked || node.Reason != "dependency_failed" {
		t.Fatalf("dependent node = %+v, want dependency_failed", node)
	}
	if events, err := k.loadEvents(); err != nil || countEvents(events, "agent_invocation.run_started") != 0 {
		t.Fatalf("failure reduction dispatched invocation: events=%+v error=%v", events, err)
	}
}

func TestTaskGraphRejectsInvocationBindingUntilNodeIsReady(t *testing.T) {
	k := newTestKernel(t, filepath.Join(testTempDir(t), "events.sqlite"))
	graph, err := k.CreateTaskGraph(TaskGraphCreateRequest{SessionID: "graph-waiting-bind"})
	if err != nil {
		t.Fatalf("create graph: %v", err)
	}
	first, err := k.AddTaskGraphNode(TaskGraphNodeRequest{GraphID: graph.GraphID})
	if err != nil {
		t.Fatalf("add first: %v", err)
	}
	second, err := k.AddTaskGraphNode(TaskGraphNodeRequest{GraphID: graph.GraphID})
	if err != nil {
		t.Fatalf("add second: %v", err)
	}
	if err := k.AddTaskGraphEdge(TaskGraphEdgeRequest{GraphID: graph.GraphID, FromNodeID: first.NodeID, ToNodeID: second.NodeID}); err != nil {
		t.Fatalf("add dependency: %v", err)
	}
	invocation, err := k.AdmitAgentInvocation(AgentInvocationAdmissionRequest{SessionID: graph.SessionID, Principal: "application:test", CapabilityGrant: CapabilityGrant{}})
	if err != nil {
		t.Fatalf("admit invocation: %v", err)
	}
	before, err := k.loadEvents()
	if err != nil {
		t.Fatalf("load before: %v", err)
	}
	if _, err := k.BindTaskGraphNodeInvocation(TaskGraphNodeBindingRequest{GraphID: graph.GraphID, NodeID: second.NodeID, InvocationID: invocation.InvocationID}); err == nil {
		t.Fatal("waiting node accepted invocation binding")
	}
	after, err := k.loadEvents()
	if err != nil || len(after) != len(before) {
		t.Fatalf("waiting binding appended fact %d -> %d error %v", len(before), len(after), err)
	}
}

func TestTaskGraphReducesAmbiguousWorkerRecoveryWithoutReplay(t *testing.T) {
	configRoot := writeParentWorkerRuntimeConfig(t, []string{"resource_read"})
	ledgerPath := filepath.Join(testTempDir(t), "events.sqlite")
	first := newAgentInvocationRunTestKernel(t, Config{LedgerPath: ledgerPath, ToolPolicy: ToolPolicy{PermissionMode: PermissionModePlan}, ParentWorkerConfigRoot: configRoot})
	graph, err := first.CreateTaskGraph(TaskGraphCreateRequest{SessionID: "graph-recovery"})
	if err != nil {
		t.Fatalf("create graph: %v", err)
	}
	node, err := first.AddTaskGraphNode(TaskGraphNodeRequest{GraphID: graph.GraphID, Title: "worker task"})
	if err != nil {
		t.Fatalf("add node: %v", err)
	}
	invocation, err := first.AdmitWorkerInvocationFromRole(WorkerInvocationAdmissionRequest{ConfigRoot: configRoot, SessionID: graph.SessionID, ParentTurnID: "turn-parent", Principal: "application:kernel", RoleID: "local-small-worker", IdempotencyKey: "evt-delegate"})
	if err != nil {
		t.Fatalf("admit worker: %v", err)
	}
	if _, err := first.BindTaskGraphNodeInvocation(TaskGraphNodeBindingRequest{GraphID: graph.GraphID, NodeID: node.NodeID, InvocationID: invocation.InvocationID}); err != nil {
		t.Fatalf("bind node: %v", err)
	}
	run := AgentInvocationRunProjection{InvocationID: invocation.InvocationID, RunID: "run-started", SessionID: invocation.SessionID, Principal: "application:kernel", Status: AgentInvocationRunStatusRunning, IdempotencyKey: invocation.IdempotencyKey, StartedAt: time.Now().UTC()}
	if err := first.appendAgentInvocationRunEvent("agent_invocation.run_started", run); err != nil {
		t.Fatalf("append started run: %v", err)
	}
	first.Close()

	child := &delegateWorkerChildProvider{completed: make(chan ModelRequest, 1)}
	restarted := newAgentInvocationRunTestKernel(t, Config{LedgerPath: ledgerPath, ToolPolicy: ToolPolicy{PermissionMode: PermissionModePlan}, ParentWorkerConfigRoot: configRoot, WorkerProviderResolver: func(string) (Provider, error) { return child, nil }})
	projection, err := restarted.TaskGraph(graph.GraphID)
	if err != nil {
		t.Fatalf("read recovered graph: %v", err)
	}
	bound := taskGraphNodeByID(t, projection, node.NodeID)
	if bound.Status != TaskGraphNodeStatusFailed || bound.Reason != "invocation_failed" || len(bound.EvidenceRefs) != 1 {
		t.Fatalf("recovered node = %+v, want failed evidence", bound)
	}
	select {
	case <-child.completed:
		t.Fatal("ambiguous worker was replayed")
	default:
	}
}

func TestTaskGraphFailsGenericRunningInvocationAcrossRestartWithoutReplay(t *testing.T) {
	ledgerPath := filepath.Join(testTempDir(t), "events.sqlite")
	first := newTestKernel(t, ledgerPath)
	graph, err := first.CreateTaskGraph(TaskGraphCreateRequest{SessionID: "graph-generic-recovery"})
	if err != nil {
		t.Fatalf("create graph: %v", err)
	}
	node, err := first.AddTaskGraphNode(TaskGraphNodeRequest{GraphID: graph.GraphID})
	if err != nil {
		t.Fatalf("add node: %v", err)
	}
	invocation, err := first.AdmitAgentInvocation(AgentInvocationAdmissionRequest{SessionID: graph.SessionID, Principal: "application:test", CapabilityGrant: CapabilityGrant{}})
	if err != nil {
		t.Fatalf("admit invocation: %v", err)
	}
	if _, err := first.BindTaskGraphNodeInvocation(TaskGraphNodeBindingRequest{GraphID: graph.GraphID, NodeID: node.NodeID, InvocationID: invocation.InvocationID}); err != nil {
		t.Fatalf("bind node: %v", err)
	}
	run := AgentInvocationRunProjection{InvocationID: invocation.InvocationID, RunID: "run-generic-started", SessionID: invocation.SessionID, Principal: "application:test", Status: AgentInvocationRunStatusRunning, StartedAt: time.Now().UTC()}
	if err := first.appendAgentInvocationRunEvent("agent_invocation.run_started", run); err != nil {
		t.Fatalf("append started run: %v", err)
	}
	first.Close()

	restarted := newTestKernel(t, ledgerPath)
	projection, err := restarted.TaskGraph(graph.GraphID)
	if err != nil {
		t.Fatalf("read recovered graph: %v", err)
	}
	bound := taskGraphNodeByID(t, projection, node.NodeID)
	if bound.Status != TaskGraphNodeStatusFailed || bound.Reason != "invocation_failed" || len(bound.EvidenceRefs) != 1 {
		t.Fatalf("generic recovered node = %+v, want failed evidence", bound)
	}
}

func TestTaskGraphFailsLatestAmbiguousRunAfterPriorTerminalRunAcrossRestart(t *testing.T) {
	ledgerPath := filepath.Join(testTempDir(t), "events.sqlite")
	first := newTestKernel(t, ledgerPath)
	graph, err := first.CreateTaskGraph(TaskGraphCreateRequest{SessionID: "graph-latest-recovery"})
	if err != nil {
		t.Fatalf("create graph: %v", err)
	}
	node, err := first.AddTaskGraphNode(TaskGraphNodeRequest{GraphID: graph.GraphID})
	if err != nil {
		t.Fatalf("add node: %v", err)
	}
	invocation, err := first.AdmitAgentInvocation(AgentInvocationAdmissionRequest{SessionID: graph.SessionID, Principal: "application:test", CapabilityGrant: CapabilityGrant{}})
	if err != nil {
		t.Fatalf("admit invocation: %v", err)
	}
	if _, err := first.BindTaskGraphNodeInvocation(TaskGraphNodeBindingRequest{GraphID: graph.GraphID, NodeID: node.NodeID, InvocationID: invocation.InvocationID}); err != nil {
		t.Fatalf("bind node: %v", err)
	}
	completed := AgentInvocationRunProjection{InvocationID: invocation.InvocationID, RunID: "run-completed", SessionID: invocation.SessionID, Principal: "application:test", Status: AgentInvocationRunStatusCompleted, CompletedAt: time.Now().UTC()}
	if err := first.appendAgentInvocationRunEvent("agent_invocation.run_completed", completed); err != nil {
		t.Fatalf("append completed run: %v", err)
	}
	running := AgentInvocationRunProjection{InvocationID: invocation.InvocationID, RunID: "run-later-started", SessionID: invocation.SessionID, Principal: "application:test", Status: AgentInvocationRunStatusRunning, StartedAt: time.Now().UTC().Add(time.Second)}
	if err := first.appendAgentInvocationRunEvent("agent_invocation.run_started", running); err != nil {
		t.Fatalf("append later started run: %v", err)
	}
	first.Close()

	restarted := newTestKernel(t, ledgerPath)
	latest, _, found, err := restarted.latestAgentInvocationRunEvent(invocation.InvocationID)
	if err != nil || !found || latest.Status != AgentInvocationRunStatusFailed || latest.Error == nil || latest.Error.Code != "agent_invocation_recovery_ambiguous" {
		t.Fatalf("latest recovered run = %+v found=%v error=%v", latest, found, err)
	}
}

func TestTaskGraphMutatesUnstartedTopologyWithoutRewritingTerminalEvidence(t *testing.T) {
	ledgerPath := filepath.Join(testTempDir(t), "events.sqlite")
	k := newTestKernel(t, ledgerPath)
	graph, err := k.CreateTaskGraph(TaskGraphCreateRequest{SessionID: "graph-mutable"})
	if err != nil {
		t.Fatalf("create graph: %v", err)
	}
	first, err := k.AddTaskGraphNode(TaskGraphNodeRequest{GraphID: graph.GraphID, Title: "initial task"})
	if err != nil {
		t.Fatalf("add first node: %v", err)
	}
	second, err := k.AddTaskGraphNode(TaskGraphNodeRequest{GraphID: graph.GraphID, Title: "dependent task"})
	if err != nil {
		t.Fatalf("add second node: %v", err)
	}
	if err := k.AddTaskGraphEdge(TaskGraphEdgeRequest{GraphID: graph.GraphID, FromNodeID: first.NodeID, ToNodeID: second.NodeID}); err != nil {
		t.Fatalf("add edge: %v", err)
	}
	if err := k.UpdateTaskGraphNode(TaskGraphNodeUpdateRequest{GraphID: graph.GraphID, NodeID: first.NodeID, Title: "refined task", Description: "discovered while exploring"}); err != nil {
		t.Fatalf("update unstarted node: %v", err)
	}
	if err := k.RemoveTaskGraphEdge(TaskGraphEdgeRemoveRequest{GraphID: graph.GraphID, FromNodeID: first.NodeID, ToNodeID: second.NodeID}); err != nil {
		t.Fatalf("remove pending edge: %v", err)
	}
	projection, err := k.TaskGraph(graph.GraphID)
	if err != nil {
		t.Fatalf("read graph: %v", err)
	}
	if node := taskGraphNodeByID(t, projection, first.NodeID); node.Title != "refined task" || node.Description != "discovered while exploring" {
		t.Fatalf("updated node = %+v", node)
	}
	if len(projection.Edges) != 0 || taskGraphNodeByID(t, projection, second.NodeID).Status != TaskGraphNodeStatusReady {
		t.Fatalf("mutable topology = %+v, want no edges and ready second node", projection)
	}
	k.Close()
	k = newTestKernel(t, ledgerPath)
	projection, err = k.TaskGraph(graph.GraphID)
	if err != nil || len(projection.Edges) != 0 || taskGraphNodeByID(t, projection, first.NodeID).Title != "refined task" {
		t.Fatalf("restarted mutable graph = %+v error = %v", projection, err)
	}
	if err := k.TransitionTaskGraphNode(TaskGraphNodeTransitionRequest{GraphID: graph.GraphID, NodeID: first.NodeID, Status: TaskGraphNodeStatusCompleted, EvidenceRefs: []string{"event:completed"}}); err != nil {
		t.Fatalf("complete first node: %v", err)
	}
	before, err := k.loadEvents()
	if err != nil {
		t.Fatalf("load before terminal mutation: %v", err)
	}
	if err := k.UpdateTaskGraphNode(TaskGraphNodeUpdateRequest{GraphID: graph.GraphID, NodeID: first.NodeID, Title: "rewritten"}); err == nil {
		t.Fatal("terminal node update accepted")
	}
	after, err := k.loadEvents()
	if err != nil || len(after) != len(before) {
		t.Fatalf("terminal mutation appended event %d -> %d error %v", len(before), len(after), err)
	}
}

func TestTaskGraphRejectsCycleAndTerminalTransitionWithoutAppendingFact(t *testing.T) {
	k := newTestKernel(t, filepath.Join(testTempDir(t), "events.sqlite"))
	graph, err := k.CreateTaskGraph(TaskGraphCreateRequest{SessionID: "graph-reject"})
	if err != nil {
		t.Fatalf("create graph: %v", err)
	}
	node, err := k.AddTaskGraphNode(TaskGraphNodeRequest{GraphID: graph.GraphID})
	if err != nil {
		t.Fatalf("add node: %v", err)
	}
	before, err := k.loadEvents()
	if err != nil {
		t.Fatalf("load before missing reference: %v", err)
	}
	if _, err := k.BindTaskGraphNodeInvocation(TaskGraphNodeBindingRequest{GraphID: graph.GraphID, NodeID: node.NodeID, InvocationID: "invocation_missing"}); err == nil {
		t.Fatal("missing invocation accepted")
	}
	after, err := k.loadEvents()
	if err != nil || len(after) != len(before) {
		t.Fatalf("missing reference appended event %d -> %d error %v", len(before), len(after), err)
	}
	firstNode, err := k.AddTaskGraphNode(TaskGraphNodeRequest{GraphID: graph.GraphID})
	if err != nil {
		t.Fatalf("add first node: %v", err)
	}
	secondNode, err := k.AddTaskGraphNode(TaskGraphNodeRequest{GraphID: graph.GraphID})
	if err != nil {
		t.Fatalf("add second node: %v", err)
	}
	if err := k.AddTaskGraphEdge(TaskGraphEdgeRequest{GraphID: graph.GraphID, FromNodeID: firstNode.NodeID, ToNodeID: secondNode.NodeID}); err != nil {
		t.Fatalf("add edge: %v", err)
	}
	before, err = k.loadEvents()
	if err != nil {
		t.Fatalf("load before: %v", err)
	}
	if err := k.AddTaskGraphEdge(TaskGraphEdgeRequest{GraphID: graph.GraphID, FromNodeID: secondNode.NodeID, ToNodeID: firstNode.NodeID}); err == nil {
		t.Fatal("cycle edge accepted")
	}
	after, err = k.loadEvents()
	if err != nil || len(after) != len(before) {
		t.Fatalf("cycle append changed events %d -> %d error %v", len(before), len(after), err)
	}
	if err := k.TransitionTaskGraphNode(TaskGraphNodeTransitionRequest{GraphID: graph.GraphID, NodeID: firstNode.NodeID, Status: TaskGraphNodeStatusCompleted}); err != nil {
		t.Fatalf("complete node: %v", err)
	}
	before, _ = k.loadEvents()
	if err := k.TransitionTaskGraphNode(TaskGraphNodeTransitionRequest{GraphID: graph.GraphID, NodeID: firstNode.NodeID, Status: TaskGraphNodeStatusCancelled}); err == nil {
		t.Fatal("terminal transition accepted")
	}
	after, _ = k.loadEvents()
	if len(after) != len(before) {
		t.Fatalf("terminal transition appended event %d -> %d", len(before), len(after))
	}
}

func TestTaskGraphFailureBlocksDependentWithReason(t *testing.T) {
	k := newTestKernel(t, filepath.Join(testTempDir(t), "events.sqlite"))
	graph, _ := k.CreateTaskGraph(TaskGraphCreateRequest{SessionID: "graph-blocked"})
	firstNode, _ := k.AddTaskGraphNode(TaskGraphNodeRequest{GraphID: graph.GraphID})
	secondNode, _ := k.AddTaskGraphNode(TaskGraphNodeRequest{GraphID: graph.GraphID})
	if err := k.AddTaskGraphEdge(TaskGraphEdgeRequest{GraphID: graph.GraphID, FromNodeID: firstNode.NodeID, ToNodeID: secondNode.NodeID}); err != nil {
		t.Fatalf("add edge: %v", err)
	}
	if err := k.TransitionTaskGraphNode(TaskGraphNodeTransitionRequest{GraphID: graph.GraphID, NodeID: firstNode.NodeID, Status: TaskGraphNodeStatusRunning}); err != nil {
		t.Fatalf("start node: %v", err)
	}
	if err := k.TransitionTaskGraphNode(TaskGraphNodeTransitionRequest{GraphID: graph.GraphID, NodeID: firstNode.NodeID, Status: TaskGraphNodeStatusFailed}); err != nil {
		t.Fatalf("fail node: %v", err)
	}
	projection, err := k.TaskGraph(graph.GraphID)
	if err != nil {
		t.Fatalf("TaskGraph: %v", err)
	}
	dependent := taskGraphNodeByID(t, projection, secondNode.NodeID)
	if dependent.Status != TaskGraphNodeStatusBlocked || dependent.Reason != "dependency_failed" {
		t.Fatalf("dependent = %+v, want blocked dependency_failed", dependent)
	}
}

func TestTaskGraphTerminalTransitionPersistsEvidenceRefs(t *testing.T) {
	k := newTestKernel(t, filepath.Join(testTempDir(t), "events.sqlite"))
	invocation, err := k.AdmitAgentInvocation(AgentInvocationAdmissionRequest{SessionID: "graph-evidence", Principal: "application:test", CapabilityGrant: CapabilityGrant{ToolNames: []string{"resource_read"}}})
	if err != nil {
		t.Fatalf("admit: %v", err)
	}
	graph, err := k.CreateTaskGraph(TaskGraphCreateRequest{SessionID: "graph-evidence"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	node, err := k.AddTaskGraphNode(TaskGraphNodeRequest{GraphID: graph.GraphID})
	if err != nil {
		t.Fatalf("add node: %v", err)
	}
	if _, err := k.BindTaskGraphNodeInvocation(TaskGraphNodeBindingRequest{GraphID: graph.GraphID, NodeID: node.NodeID, InvocationID: invocation.InvocationID}); err != nil {
		t.Fatalf("bind node: %v", err)
	}
	if err := k.TransitionTaskGraphNode(TaskGraphNodeTransitionRequest{GraphID: graph.GraphID, NodeID: node.NodeID, Status: TaskGraphNodeStatusCompleted, EvidenceRefs: []string{"event:worker-final"}}); err != nil {
		t.Fatalf("complete: %v", err)
	}
	projection, err := k.TaskGraph(graph.GraphID)
	if err != nil || len(taskGraphNodeByID(t, projection, node.NodeID).EvidenceRefs) != 1 || taskGraphNodeByID(t, projection, node.NodeID).EvidenceRefs[0] != "event:worker-final" {
		t.Fatalf("node evidence = %+v error = %v", taskGraphNodeByID(t, projection, node.NodeID), err)
	}
}

func taskGraphNodeByID(t *testing.T, graph TaskGraphProjection, nodeID string) TaskGraphNodeProjection {
	t.Helper()
	for _, node := range graph.Nodes {
		if node.NodeID == nodeID {
			return node
		}
	}
	t.Fatalf("node %q missing from %+v", nodeID, graph)
	return TaskGraphNodeProjection{}
}
