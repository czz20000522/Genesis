package workflowruntime

import (
	"strings"
	"testing"
)

func TestCompile_CanonicalHashIgnoresSourceMapOrder(t *testing.T) {
	t.Parallel()

	first, err := Compile([]byte(`
workflow_id: transcript_cleanup
initial_node_id: extract
nodes:
  extract:
    kind: machine_check
    executor_ref: extract
    allowed_outcomes: [pass]
  clean:
    kind: machine_check
    executor_ref: clean
    allowed_outcomes: [done]
edges:
  - from: extract
    outcome: pass
    to: clean
terminal_outcomes:
  done: success
`))
	if err != nil {
		t.Fatalf("Compile(first) error = %v", err)
	}
	second, err := Compile([]byte(`
terminal_outcomes:
  done: success
edges:
  - to: clean
    outcome: pass
    from: extract
nodes:
  clean:
    allowed_outcomes: [done]
    executor_ref: clean
    kind: machine_check
  extract:
    allowed_outcomes: [pass]
    kind: machine_check
    executor_ref: extract
initial_node_id: extract
workflow_id: transcript_cleanup
`))
	if err != nil {
		t.Fatalf("Compile(second) error = %v", err)
	}
	if first.DefinitionHash != second.DefinitionHash {
		t.Fatalf("hashes differ: %q != %q", first.DefinitionHash, second.DefinitionHash)
	}
}

func TestCompile_RejectsUndeclaredEdgeOutcome(t *testing.T) {
	t.Parallel()

	_, err := Compile([]byte(`
workflow_id: transcript_cleanup
initial_node_id: extract
nodes:
  extract:
    kind: machine_check
    executor_ref: extract
    allowed_outcomes: [pass]
edges:
  - from: extract
    outcome: fail
    to: done
terminal_outcomes:
  done: success
`))
	if err == nil || !strings.Contains(err.Error(), "undeclared outcome") {
		t.Fatalf("Compile() error = %v, want undeclared outcome", err)
	}
}

func TestCompile_RejectsCycleWithoutLoopLimit(t *testing.T) {
	t.Parallel()

	_, err := Compile([]byte(`
workflow_id: transcript_cleanup
initial_node_id: extract
nodes:
  extract:
    kind: machine_check
    executor_ref: extract
    allowed_outcomes: [retry]
edges:
  - from: extract
    outcome: retry
    to: extract
terminal_outcomes: {}
`))
	if err == nil || !strings.Contains(err.Error(), "loop limit") {
		t.Fatalf("Compile() error = %v, want loop limit", err)
	}
}

func TestCompile_RejectsExecutableConfigField(t *testing.T) {
	t.Parallel()

	_, err := Compile([]byte(`
workflow_id: transcript_cleanup
initial_node_id: extract
nodes:
  extract:
    kind: machine_check
    executor_ref: extract
    command: powershell -File extract.ps1
    allowed_outcomes: [done]
terminal_outcomes:
  done: success
`))
	if err == nil || !strings.Contains(err.Error(), "command") {
		t.Fatalf("Compile() error = %v, want rejected command field", err)
	}
}

func TestCompile_GeneratesFlowchartForEdgesAndTerminals(t *testing.T) {
	t.Parallel()

	definition, err := Compile([]byte(`
workflow_id: transcript_cleanup
initial_node_id: extract
nodes:
  extract:
    kind: machine_check
    executor_ref: extract
    allowed_outcomes: [pass, blocked]
  clean:
    kind: machine_check
    executor_ref: clean
    allowed_outcomes: [done]
edges:
  - from: extract
    outcome: pass
    to: clean
terminal_outcomes:
  done: success
  blocked: blocked
`))
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}
	for _, want := range []string{"extract", "clean", "pass", "done", "blocked"} {
		if !strings.Contains(definition.Flowchart, want) {
			t.Fatalf("flowchart %q does not contain %q", definition.Flowchart, want)
		}
	}
}
