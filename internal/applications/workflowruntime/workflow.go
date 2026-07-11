// Package workflowruntime compiles fixed user-space workflow definitions.
package workflowruntime

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
	"unicode"

	"gopkg.in/yaml.v3"
)

type config struct {
	WorkflowID       string                `yaml:"workflow_id"`
	DisplayName      string                `yaml:"display_name"`
	InitialNodeID    string                `yaml:"initial_node_id"`
	Nodes            map[string]nodeConfig `yaml:"nodes"`
	Edges            []edge                `yaml:"edges"`
	TerminalOutcomes map[string]string     `yaml:"terminal_outcomes"`
	Limits           limits                `yaml:"limits"`
}

type nodeConfig struct {
	Kind            string   `yaml:"kind"`
	ExecutorRef     string   `yaml:"executor_ref"`
	AllowedOutcomes []string `yaml:"allowed_outcomes"`
}

type limits struct {
	MaxLoopCount int `yaml:"max_loop_count"`
}

// Definition is the fixed, normalized contract admitted by a future runner.
type Definition struct {
	WorkflowID       string            `json:"workflow_id"`
	DisplayName      string            `json:"display_name,omitempty"`
	InitialNodeID    string            `json:"initial_node_id"`
	Nodes            []Node            `json:"nodes"`
	Edges            []Edge            `json:"edges"`
	TerminalOutcomes []TerminalOutcome `json:"terminal_outcomes"`
	MaxLoopCount     int               `json:"max_loop_count"`
	DefinitionHash   string            `json:"definition_hash"`
	Flowchart        string            `json:"flowchart"`
}

type Node struct {
	NodeID          string   `json:"node_id"`
	Kind            string   `json:"kind"`
	ExecutorRef     string   `json:"executor_ref"`
	AllowedOutcomes []string `json:"allowed_outcomes"`
}

type Edge struct {
	From    string `yaml:"from" json:"from"`
	Outcome string `yaml:"outcome" json:"outcome"`
	To      string `yaml:"to" json:"to"`
}

type TerminalOutcome struct {
	Name   string `json:"name"`
	Status string `json:"status"`
}

type edge = Edge

// Compile parses a JSON or YAML config and returns its canonical fixed graph.
func Compile(source []byte) (Definition, error) {
	var input config
	decoder := yaml.NewDecoder(bytes.NewReader(source))
	decoder.KnownFields(true)
	if err := decoder.Decode(&input); err != nil {
		return Definition{}, fmt.Errorf("decode workflow config: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		if err == nil {
			return Definition{}, fmt.Errorf("workflow config contains multiple documents")
		}
		return Definition{}, fmt.Errorf("decode workflow config: %w", err)
	}
	return compile(input)
}

func compile(input config) (Definition, error) {
	workflowID := strings.TrimSpace(input.WorkflowID)
	initialNodeID := strings.TrimSpace(input.InitialNodeID)
	if !validID(workflowID) {
		return Definition{}, fmt.Errorf("invalid workflow id %q", workflowID)
	}
	if !validID(initialNodeID) {
		return Definition{}, fmt.Errorf("invalid initial node id %q", initialNodeID)
	}
	if len(input.Nodes) == 0 {
		return Definition{}, fmt.Errorf("workflow has no nodes")
	}

	definition := Definition{
		WorkflowID:    workflowID,
		DisplayName:   strings.TrimSpace(input.DisplayName),
		InitialNodeID: initialNodeID,
		MaxLoopCount:  input.Limits.MaxLoopCount,
	}
	nodeByID := make(map[string]Node, len(input.Nodes))
	for nodeID, rawNode := range input.Nodes {
		node, err := normalizeNode(nodeID, rawNode)
		if err != nil {
			return Definition{}, err
		}
		nodeByID[node.NodeID] = node
		definition.Nodes = append(definition.Nodes, node)
	}
	if _, ok := nodeByID[initialNodeID]; !ok {
		return Definition{}, fmt.Errorf("initial node %q is undeclared", initialNodeID)
	}
	sort.Slice(definition.Nodes, func(i, j int) bool { return definition.Nodes[i].NodeID < definition.Nodes[j].NodeID })

	terminals := make(map[string]struct{}, len(input.TerminalOutcomes))
	for name, status := range input.TerminalOutcomes {
		name = strings.TrimSpace(name)
		if !validID(name) || strings.TrimSpace(status) == "" {
			return Definition{}, fmt.Errorf("invalid terminal outcome %q", name)
		}
		terminals[name] = struct{}{}
		definition.TerminalOutcomes = append(definition.TerminalOutcomes, TerminalOutcome{Name: name, Status: strings.TrimSpace(status)})
	}
	sort.Slice(definition.TerminalOutcomes, func(i, j int) bool { return definition.TerminalOutcomes[i].Name < definition.TerminalOutcomes[j].Name })

	edges := make(map[string]struct{}, len(input.Edges))
	for _, rawEdge := range input.Edges {
		edge := Edge{From: strings.TrimSpace(rawEdge.From), Outcome: strings.TrimSpace(rawEdge.Outcome), To: strings.TrimSpace(rawEdge.To)}
		from, ok := nodeByID[edge.From]
		if !ok {
			return Definition{}, fmt.Errorf("edge from undeclared node %q", edge.From)
		}
		if !contains(from.AllowedOutcomes, edge.Outcome) {
			return Definition{}, fmt.Errorf("edge from %q uses undeclared outcome %q", edge.From, edge.Outcome)
		}
		if _, ok := nodeByID[edge.To]; !ok {
			if _, terminal := terminals[edge.To]; !terminal {
				return Definition{}, fmt.Errorf("edge to undeclared node or terminal %q", edge.To)
			}
		}
		key := edge.From + "\x00" + edge.Outcome
		if _, duplicate := edges[key]; duplicate {
			return Definition{}, fmt.Errorf("multiple edges for %q outcome %q", edge.From, edge.Outcome)
		}
		edges[key] = struct{}{}
		definition.Edges = append(definition.Edges, edge)
	}
	sort.Slice(definition.Edges, func(i, j int) bool {
		left, right := definition.Edges[i], definition.Edges[j]
		if left.From != right.From {
			return left.From < right.From
		}
		return left.Outcome < right.Outcome
	})
	for _, node := range definition.Nodes {
		for _, outcome := range node.AllowedOutcomes {
			if _, hasEdge := edges[node.NodeID+"\x00"+outcome]; hasEdge {
				continue
			}
			if _, terminal := terminals[outcome]; !terminal {
				return Definition{}, fmt.Errorf("node %q outcome %q has no transition", node.NodeID, outcome)
			}
		}
	}
	if hasCycle(definition.Nodes, definition.Edges, nodeByID) && definition.MaxLoopCount <= 0 {
		return Definition{}, fmt.Errorf("workflow cycle requires a positive loop limit")
	}
	canonical, err := json.Marshal(struct {
		WorkflowID       string            `json:"workflow_id"`
		DisplayName      string            `json:"display_name,omitempty"`
		InitialNodeID    string            `json:"initial_node_id"`
		Nodes            []Node            `json:"nodes"`
		Edges            []Edge            `json:"edges"`
		TerminalOutcomes []TerminalOutcome `json:"terminal_outcomes"`
		MaxLoopCount     int               `json:"max_loop_count"`
	}{workflowID, definition.DisplayName, initialNodeID, definition.Nodes, definition.Edges, definition.TerminalOutcomes, definition.MaxLoopCount})
	if err != nil {
		return Definition{}, fmt.Errorf("marshal workflow definition: %w", err)
	}
	sum := sha256.Sum256(canonical)
	definition.DefinitionHash = hex.EncodeToString(sum[:])
	definition.Flowchart = flowchart(definition, terminals, edges)
	return definition, nil
}

func normalizeNode(id string, raw nodeConfig) (Node, error) {
	id = strings.TrimSpace(id)
	if !validID(id) || !validID(strings.TrimSpace(raw.ExecutorRef)) || strings.TrimSpace(raw.Kind) == "" {
		return Node{}, fmt.Errorf("invalid node %q", id)
	}
	if len(raw.AllowedOutcomes) == 0 {
		return Node{}, fmt.Errorf("node %q has no allowed outcomes", id)
	}
	seen := make(map[string]struct{}, len(raw.AllowedOutcomes))
	outcomes := make([]string, 0, len(raw.AllowedOutcomes))
	for _, outcome := range raw.AllowedOutcomes {
		outcome = strings.TrimSpace(outcome)
		if !validID(outcome) {
			return Node{}, fmt.Errorf("invalid outcome %q for node %q", outcome, id)
		}
		if _, duplicate := seen[outcome]; duplicate {
			return Node{}, fmt.Errorf("duplicate outcome %q for node %q", outcome, id)
		}
		seen[outcome] = struct{}{}
		outcomes = append(outcomes, outcome)
	}
	sort.Strings(outcomes)
	return Node{NodeID: id, Kind: strings.TrimSpace(raw.Kind), ExecutorRef: strings.TrimSpace(raw.ExecutorRef), AllowedOutcomes: outcomes}, nil
}

func hasCycle(nodes []Node, edges []Edge, nodeByID map[string]Node) bool {
	adjacent := make(map[string][]string, len(nodes))
	for _, edge := range edges {
		if _, ok := nodeByID[edge.To]; ok {
			adjacent[edge.From] = append(adjacent[edge.From], edge.To)
		}
	}
	state := make(map[string]uint8, len(nodes))
	var visit func(string) bool
	visit = func(nodeID string) bool {
		state[nodeID] = 1
		for _, next := range adjacent[nodeID] {
			if state[next] == 1 || state[next] == 0 && visit(next) {
				return true
			}
		}
		state[nodeID] = 2
		return false
	}
	for _, node := range nodes {
		if state[node.NodeID] == 0 && visit(node.NodeID) {
			return true
		}
	}
	return false
}

func flowchart(definition Definition, terminals map[string]struct{}, edges map[string]struct{}) string {
	var builder strings.Builder
	builder.WriteString("flowchart TD\n")
	for _, node := range definition.Nodes {
		fmt.Fprintf(&builder, "  %s[%q]\n", node.NodeID, node.NodeID)
	}
	for _, terminal := range definition.TerminalOutcomes {
		fmt.Fprintf(&builder, "  terminal_%s([%q])\n", terminal.Name, terminal.Name+": "+terminal.Status)
	}
	for _, edge := range definition.Edges {
		to := edge.To
		if _, terminal := terminals[to]; terminal {
			to = "terminal_" + to
		}
		fmt.Fprintf(&builder, "  %s -- %s --> %s\n", edge.From, edge.Outcome, to)
	}
	for _, node := range definition.Nodes {
		for _, outcome := range node.AllowedOutcomes {
			if _, hasEdge := edges[node.NodeID+"\x00"+outcome]; !hasEdge {
				fmt.Fprintf(&builder, "  %s -- %s --> terminal_%s\n", node.NodeID, outcome, outcome)
			}
		}
	}
	return builder.String()
}

func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func validID(value string) bool {
	if value == "" {
		return false
	}
	for index, runeValue := range value {
		if index == 0 {
			if !unicode.IsLetter(runeValue) {
				return false
			}
			continue
		}
		if !unicode.IsLetter(runeValue) && !unicode.IsDigit(runeValue) && runeValue != '_' && runeValue != '-' {
			return false
		}
	}
	return true
}
