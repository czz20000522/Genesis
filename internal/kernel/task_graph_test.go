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
	node, err := k.AddTaskGraphNode(TaskGraphNodeRequest{GraphID: graph.GraphID, InvocationID: invocation.InvocationID})
	if err != nil {
		t.Fatalf("add node: %v", err)
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
