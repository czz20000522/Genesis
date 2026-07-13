package kernel

import "time"

const (
	TaskGraphNodeStatusProposed  = "proposed"
	TaskGraphNodeStatusReady     = "ready"
	TaskGraphNodeStatusRunning   = "running"
	TaskGraphNodeStatusWaiting   = "waiting"
	TaskGraphNodeStatusBlocked   = "blocked"
	TaskGraphNodeStatusCompleted = "completed"
	TaskGraphNodeStatusFailed    = "failed"
	TaskGraphNodeStatusCancelled = "cancelled"
)

type TaskGraphCreateRequest struct {
	SessionID string `json:"session_id"`
}
type TaskGraphNodeRequest struct {
	GraphID      string `json:"graph_id"`
	InvocationID string `json:"invocation_id"`
	Title        string `json:"title,omitempty"`
	Description  string `json:"description,omitempty"`
}
type TaskGraphNodeUpdateRequest struct {
	GraphID     string `json:"graph_id"`
	NodeID      string `json:"node_id"`
	Title       string `json:"title"`
	Description string `json:"description"`
}
type TaskGraphNodeBindingRequest struct {
	GraphID      string `json:"graph_id"`
	NodeID       string `json:"node_id"`
	InvocationID string `json:"invocation_id"`
}
type TaskGraphEdgeRequest struct {
	GraphID    string `json:"graph_id"`
	FromNodeID string `json:"from_node_id"`
	ToNodeID   string `json:"to_node_id"`
}
type TaskGraphEdgeRemoveRequest struct {
	GraphID    string `json:"graph_id"`
	FromNodeID string `json:"from_node_id"`
	ToNodeID   string `json:"to_node_id"`
}
type TaskGraphNodeTransitionRequest struct {
	GraphID      string   `json:"graph_id"`
	NodeID       string   `json:"node_id"`
	Status       string   `json:"status"`
	Reason       string   `json:"reason,omitempty"`
	EvidenceRefs []string `json:"evidence_refs,omitempty"`
}
type TaskGraphNodeProjection struct {
	NodeID       string    `json:"node_id"`
	InvocationID string    `json:"invocation_id"`
	Title        string    `json:"title,omitempty"`
	Description  string    `json:"description,omitempty"`
	Status       string    `json:"status"`
	Reason       string    `json:"reason,omitempty"`
	EvidenceRefs []string  `json:"evidence_refs,omitempty"`
	UpdatedAt    time.Time `json:"updated_at"`
}
type TaskGraphEdgeProjection struct {
	FromNodeID string `json:"from_node_id"`
	ToNodeID   string `json:"to_node_id"`
}
type TaskGraphProjection struct {
	GraphID   string                    `json:"graph_id"`
	SessionID string                    `json:"session_id"`
	Nodes     []TaskGraphNodeProjection `json:"nodes"`
	Edges     []TaskGraphEdgeProjection `json:"edges"`
	CreatedAt time.Time                 `json:"created_at"`
}
type TaskGraphEventProjection struct {
	GraphID   string                   `json:"graph_id"`
	SessionID string                   `json:"session_id"`
	Node      *TaskGraphNodeProjection `json:"node,omitempty"`
	Edge      *TaskGraphEdgeProjection `json:"edge,omitempty"`
	CreatedAt time.Time                `json:"created_at"`
}
