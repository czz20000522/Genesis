package kernel

import (
	"path/filepath"
	"testing"
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
	firstNode, err := k.AddTaskGraphNode(TaskGraphNodeRequest{GraphID: graph.GraphID, InvocationID: first.InvocationID})
	if err != nil {
		t.Fatalf("AddTaskGraphNode first: %v", err)
	}
	secondNode, err := k.AddTaskGraphNode(TaskGraphNodeRequest{GraphID: graph.GraphID, InvocationID: second.InvocationID})
	if err != nil {
		t.Fatalf("AddTaskGraphNode second: %v", err)
	}
	if err := k.AddTaskGraphEdge(TaskGraphEdgeRequest{GraphID: graph.GraphID, FromNodeID: firstNode.NodeID, ToNodeID: secondNode.NodeID}); err != nil {
		t.Fatalf("AddTaskGraphEdge: %v", err)
	}
	if err := k.TransitionTaskGraphNode(TaskGraphNodeTransitionRequest{GraphID: graph.GraphID, NodeID: firstNode.NodeID, Status: TaskGraphNodeStatusCompleted}); err != nil {
		t.Fatalf("complete first node: %v", err)
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

func TestTaskGraphRejectsCycleAndTerminalTransitionWithoutAppendingFact(t *testing.T) {
	k := newTestKernel(t, filepath.Join(testTempDir(t), "events.sqlite"))
	first, err := k.AdmitAgentInvocation(AgentInvocationAdmissionRequest{SessionID: "graph-reject", Principal: "application:test", CapabilityGrant: CapabilityGrant{ToolNames: []string{"resource_read"}}})
	if err != nil {
		t.Fatalf("admit first: %v", err)
	}
	second, err := k.AdmitAgentInvocation(AgentInvocationAdmissionRequest{SessionID: "graph-reject", Principal: "application:test", CapabilityGrant: CapabilityGrant{ToolNames: []string{"resource_read"}}})
	if err != nil {
		t.Fatalf("admit second: %v", err)
	}
	graph, err := k.CreateTaskGraph(TaskGraphCreateRequest{SessionID: "graph-reject"})
	if err != nil {
		t.Fatalf("create graph: %v", err)
	}
	before, err := k.loadEvents()
	if err != nil {
		t.Fatalf("load before missing reference: %v", err)
	}
	if _, err := k.AddTaskGraphNode(TaskGraphNodeRequest{GraphID: graph.GraphID, InvocationID: "invocation_missing"}); err == nil {
		t.Fatal("missing invocation accepted")
	}
	after, err := k.loadEvents()
	if err != nil || len(after) != len(before) {
		t.Fatalf("missing reference appended event %d -> %d error %v", len(before), len(after), err)
	}
	firstNode, err := k.AddTaskGraphNode(TaskGraphNodeRequest{GraphID: graph.GraphID, InvocationID: first.InvocationID})
	if err != nil {
		t.Fatalf("add first node: %v", err)
	}
	secondNode, err := k.AddTaskGraphNode(TaskGraphNodeRequest{GraphID: graph.GraphID, InvocationID: second.InvocationID})
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
	first, _ := k.AdmitAgentInvocation(AgentInvocationAdmissionRequest{SessionID: "graph-blocked", Principal: "application:test", CapabilityGrant: CapabilityGrant{ToolNames: []string{"resource_read"}}})
	second, _ := k.AdmitAgentInvocation(AgentInvocationAdmissionRequest{SessionID: "graph-blocked", Principal: "application:test", CapabilityGrant: CapabilityGrant{ToolNames: []string{"resource_read"}}})
	graph, _ := k.CreateTaskGraph(TaskGraphCreateRequest{SessionID: "graph-blocked"})
	firstNode, _ := k.AddTaskGraphNode(TaskGraphNodeRequest{GraphID: graph.GraphID, InvocationID: first.InvocationID})
	secondNode, _ := k.AddTaskGraphNode(TaskGraphNodeRequest{GraphID: graph.GraphID, InvocationID: second.InvocationID})
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
