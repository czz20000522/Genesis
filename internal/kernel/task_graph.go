package kernel

import (
	"errors"
	"sort"
	"strings"
)

var ErrTaskGraphNotFound = errors.New("task graph not found")

func (k *Kernel) CreateTaskGraph(req TaskGraphCreateRequest) (TaskGraphProjection, error) {
	if strings.TrimSpace(req.SessionID) == "" {
		return TaskGraphProjection{}, errors.New("session_id is required")
	}
	k.taskGraphMu.Lock()
	defer k.taskGraphMu.Unlock()
	now := k.clock()
	graph := TaskGraphProjection{GraphID: newID("task_graph", now), SessionID: strings.TrimSpace(req.SessionID), CreatedAt: now}
	if err := k.appendTaskGraphEvent("task_graph.created", TaskGraphEventProjection{GraphID: graph.GraphID, SessionID: graph.SessionID, CreatedAt: now}); err != nil {
		return TaskGraphProjection{}, err
	}
	return graph, nil
}

func (k *Kernel) AddTaskGraphNode(req TaskGraphNodeRequest) (TaskGraphNodeProjection, error) {
	k.taskGraphMu.Lock()
	defer k.taskGraphMu.Unlock()
	graph, err := k.taskGraph(req.GraphID)
	if err != nil {
		return TaskGraphNodeProjection{}, err
	}
	invocationID := strings.TrimSpace(req.InvocationID)
	if invocationID != "" {
		invocation, err := k.AgentInvocation(invocationID)
		if err != nil {
			return TaskGraphNodeProjection{}, err
		}
		if invocation.SessionID != graph.SessionID {
			return TaskGraphNodeProjection{}, errors.New("task graph invocation session mismatch")
		}
	}
	now := k.clock()
	node := TaskGraphNodeProjection{NodeID: newID("task_node", now), InvocationID: invocationID, Title: strings.TrimSpace(req.Title), Description: strings.TrimSpace(req.Description), Status: TaskGraphNodeStatusProposed, UpdatedAt: now}
	if err := k.appendTaskGraphEvent("task_graph.node_added", TaskGraphEventProjection{GraphID: graph.GraphID, SessionID: graph.SessionID, Node: &node, CreatedAt: now}); err != nil {
		return TaskGraphNodeProjection{}, err
	}
	return node, nil
}

func (k *Kernel) AddTaskGraphEdge(req TaskGraphEdgeRequest) error {
	k.taskGraphMu.Lock()
	defer k.taskGraphMu.Unlock()
	graph, err := k.taskGraph(req.GraphID)
	if err != nil {
		return err
	}
	from, to := strings.TrimSpace(req.FromNodeID), strings.TrimSpace(req.ToNodeID)
	if from == "" || to == "" || from == to {
		return errors.New("task graph edge invalid")
	}
	fromNode, fromFound := taskGraphNode(graph, from)
	toNode, toFound := taskGraphNode(graph, to)
	if !fromFound || !toFound || !taskGraphNodeMutable(fromNode) || !taskGraphNodeMutable(toNode) || taskGraphHasEdge(graph, from, to) {
		return errors.New("task graph edge invalid")
	}
	graph.Edges = append(graph.Edges, TaskGraphEdgeProjection{FromNodeID: from, ToNodeID: to})
	if taskGraphHasCycle(graph) {
		return errors.New("task graph cycle")
	}
	now := k.clock()
	edge := TaskGraphEdgeProjection{FromNodeID: from, ToNodeID: to}
	return k.appendTaskGraphEvent("task_graph.edge_added", TaskGraphEventProjection{GraphID: graph.GraphID, SessionID: graph.SessionID, Edge: &edge, CreatedAt: now})
}

func (k *Kernel) RemoveTaskGraphEdge(req TaskGraphEdgeRemoveRequest) error {
	k.taskGraphMu.Lock()
	defer k.taskGraphMu.Unlock()
	graph, err := k.taskGraph(req.GraphID)
	if err != nil {
		return err
	}
	from, to := strings.TrimSpace(req.FromNodeID), strings.TrimSpace(req.ToNodeID)
	fromNode, fromFound := taskGraphNode(graph, from)
	toNode, toFound := taskGraphNode(graph, to)
	if !fromFound || !toFound || !taskGraphNodeMutable(fromNode) || !taskGraphNodeMutable(toNode) || !taskGraphHasEdge(graph, from, to) {
		return errors.New("task graph edge invalid")
	}
	now := k.clock()
	edge := TaskGraphEdgeProjection{FromNodeID: from, ToNodeID: to}
	return k.appendTaskGraphEvent("task_graph.edge_removed", TaskGraphEventProjection{GraphID: graph.GraphID, SessionID: graph.SessionID, Edge: &edge, CreatedAt: now})
}

func (k *Kernel) UpdateTaskGraphNode(req TaskGraphNodeUpdateRequest) error {
	k.taskGraphMu.Lock()
	defer k.taskGraphMu.Unlock()
	graph, err := k.taskGraph(req.GraphID)
	if err != nil {
		return err
	}
	node, found := taskGraphNode(graph, strings.TrimSpace(req.NodeID))
	if !found {
		return errors.New("task graph node not found")
	}
	if !taskGraphNodeMutable(node) {
		return errors.New("task graph node immutable")
	}
	now := k.clock()
	node.Title, node.Description, node.UpdatedAt = strings.TrimSpace(req.Title), strings.TrimSpace(req.Description), now
	return k.appendTaskGraphEvent("task_graph.node_updated", TaskGraphEventProjection{GraphID: graph.GraphID, SessionID: graph.SessionID, Node: &node, CreatedAt: now})
}

func (k *Kernel) TransitionTaskGraphNode(req TaskGraphNodeTransitionRequest) error {
	k.taskGraphMu.Lock()
	defer k.taskGraphMu.Unlock()
	graph, err := k.taskGraph(req.GraphID)
	if err != nil {
		return err
	}
	graph.Nodes = taskGraphProjectedNodes(graph)
	for _, node := range graph.Nodes {
		if node.NodeID == strings.TrimSpace(req.NodeID) {
			if !taskGraphTransitionAllowed(node.Status, req.Status) {
				return errors.New("task graph transition invalid")
			}
			node.Status, node.Reason, node.EvidenceRefs, node.UpdatedAt = req.Status, strings.TrimSpace(req.Reason), cloneStringSlice(req.EvidenceRefs), k.clock()
			return k.appendTaskGraphEvent("task_graph.node_transitioned", TaskGraphEventProjection{GraphID: graph.GraphID, SessionID: graph.SessionID, Node: &node, CreatedAt: node.UpdatedAt})
		}
	}
	return errors.New("task graph node not found")
}

func (k *Kernel) TaskGraph(graphID string) (TaskGraphProjection, error) { return k.taskGraph(graphID) }

func (k *Kernel) TaskGraphs(sessionID string) ([]TaskGraphProjection, error) {
	graphs, err := k.taskGraphs()
	if err != nil {
		return nil, err
	}
	sessionID = strings.TrimSpace(sessionID)
	items := make([]TaskGraphProjection, 0)
	for _, graph := range graphs {
		if graph.SessionID == sessionID {
			graph.Nodes = taskGraphProjectedNodes(graph)
			items = append(items, graph)
		}
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].CreatedAt.Equal(items[j].CreatedAt) {
			return items[i].GraphID < items[j].GraphID
		}
		return items[i].CreatedAt.Before(items[j].CreatedAt)
	})
	return items, nil
}

func (k *Kernel) taskGraph(graphID string) (TaskGraphProjection, error) {
	graphs, err := k.taskGraphs()
	if err != nil {
		return TaskGraphProjection{}, err
	}
	graph, ok := graphs[strings.TrimSpace(graphID)]
	if !ok {
		return TaskGraphProjection{}, ErrTaskGraphNotFound
	}
	graph.Nodes = taskGraphProjectedNodes(graph)
	return graph, nil
}
func (k *Kernel) taskGraphs() (map[string]TaskGraphProjection, error) {
	events, err := k.loadEvents()
	if err != nil {
		return nil, err
	}
	graphs := map[string]TaskGraphProjection{}
	for _, event := range events {
		data := event.Data.TaskGraph
		if data == nil {
			continue
		}
		graph, ok := graphs[data.GraphID]
		if event.Type == "task_graph.created" {
			graphs[data.GraphID] = TaskGraphProjection{GraphID: data.GraphID, SessionID: data.SessionID, CreatedAt: data.CreatedAt}
			continue
		}
		if !ok {
			return nil, errors.New("task graph event missing graph")
		}
		if data.Node != nil {
			replaced := false
			for i := range graph.Nodes {
				if graph.Nodes[i].NodeID == data.Node.NodeID {
					graph.Nodes[i] = *data.Node
					replaced = true
				}
			}
			if !replaced {
				graph.Nodes = append(graph.Nodes, *data.Node)
			}
		}
		if data.Edge != nil && event.Type == "task_graph.edge_added" {
			graph.Edges = append(graph.Edges, *data.Edge)
		}
		if data.Edge != nil && event.Type == "task_graph.edge_removed" {
			graph.Edges = taskGraphWithoutEdge(graph.Edges, *data.Edge)
		}
		graphs[data.GraphID] = graph
	}
	return graphs, nil
}
func (k *Kernel) appendTaskGraphEvent(eventType string, data TaskGraphEventProjection) error {
	return k.appendEvent(StoredEvent{EventID: newID("evt", k.clock()), SessionID: data.SessionID, Type: eventType, CreatedAt: data.CreatedAt, Data: EventData{TaskGraph: &data}})
}
func taskGraphNode(graph TaskGraphProjection, id string) (TaskGraphNodeProjection, bool) {
	for _, node := range graph.Nodes {
		if node.NodeID == id {
			return node, true
		}
	}
	return TaskGraphNodeProjection{}, false
}
func taskGraphHasEdge(graph TaskGraphProjection, from string, to string) bool {
	for _, edge := range graph.Edges {
		if edge.FromNodeID == from && edge.ToNodeID == to {
			return true
		}
	}
	return false
}
func taskGraphWithoutEdge(edges []TaskGraphEdgeProjection, remove TaskGraphEdgeProjection) []TaskGraphEdgeProjection {
	filtered := make([]TaskGraphEdgeProjection, 0, len(edges))
	for _, edge := range edges {
		if edge.FromNodeID != remove.FromNodeID || edge.ToNodeID != remove.ToNodeID {
			filtered = append(filtered, edge)
		}
	}
	return filtered
}
func taskGraphHasCycle(graph TaskGraphProjection) bool {
	next := map[string][]string{}
	for _, edge := range graph.Edges {
		next[edge.FromNodeID] = append(next[edge.FromNodeID], edge.ToNodeID)
	}
	seen, stack := map[string]bool{}, map[string]bool{}
	var visit func(string) bool
	visit = func(id string) bool {
		if stack[id] {
			return true
		}
		if seen[id] {
			return false
		}
		seen[id], stack[id] = true, true
		for _, child := range next[id] {
			if visit(child) {
				return true
			}
		}
		delete(stack, id)
		return false
	}
	for _, node := range graph.Nodes {
		if visit(node.NodeID) {
			return true
		}
	}
	return false
}
func taskGraphProjectedNodes(graph TaskGraphProjection) []TaskGraphNodeProjection {
	nodes := append([]TaskGraphNodeProjection(nil), graph.Nodes...)
	completed := map[string]bool{}
	for _, node := range nodes {
		completed[node.NodeID] = node.Status == TaskGraphNodeStatusCompleted
	}
	for i := range nodes {
		if !taskGraphNodeMutable(nodes[i]) {
			continue
		}
		waiting := false
		for _, edge := range graph.Edges {
			if edge.ToNodeID != nodes[i].NodeID {
				continue
			}
			for _, predecessor := range nodes {
				if predecessor.NodeID != edge.FromNodeID {
					continue
				}
				if predecessor.Status == TaskGraphNodeStatusFailed || predecessor.Status == TaskGraphNodeStatusCancelled {
					nodes[i].Status = TaskGraphNodeStatusBlocked
					nodes[i].Reason = "dependency_" + predecessor.Status
					waiting = false
					break
				}
				if !completed[predecessor.NodeID] {
					waiting = true
				}
			}
			if nodes[i].Status == TaskGraphNodeStatusBlocked {
				break
			}
		}
		if nodes[i].Status == TaskGraphNodeStatusBlocked {
			continue
		}
		if waiting {
			nodes[i].Status = TaskGraphNodeStatusWaiting
			nodes[i].Reason = ""
		} else {
			nodes[i].Status = TaskGraphNodeStatusReady
			nodes[i].Reason = ""
		}
	}
	sort.Slice(nodes, func(i, j int) bool { return nodes[i].NodeID < nodes[j].NodeID })
	return nodes
}
func taskGraphNodeMutable(node TaskGraphNodeProjection) bool {
	return node.Status != TaskGraphNodeStatusRunning && node.Status != TaskGraphNodeStatusCompleted && node.Status != TaskGraphNodeStatusFailed && node.Status != TaskGraphNodeStatusCancelled
}
func taskGraphTransitionAllowed(current string, next string) bool {
	if current == TaskGraphNodeStatusReady {
		return next == TaskGraphNodeStatusRunning || next == TaskGraphNodeStatusCompleted || next == TaskGraphNodeStatusCancelled
	}
	if current == TaskGraphNodeStatusRunning {
		return next == TaskGraphNodeStatusCompleted || next == TaskGraphNodeStatusFailed || next == TaskGraphNodeStatusWaiting || next == TaskGraphNodeStatusCancelled
	}
	return false
}
