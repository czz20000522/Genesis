package kernel

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
)

func TestHTTPTaskGraphReadsAreSessionScoped(t *testing.T) {
	k := newTestKernel(t, filepath.Join(testTempDir(t), "events.sqlite"))
	first, err := k.CreateTaskGraph(TaskGraphCreateRequest{SessionID: "task-graph-http-a"})
	if err != nil {
		t.Fatalf("create first graph: %v", err)
	}
	if _, err := k.CreateTaskGraph(TaskGraphCreateRequest{SessionID: "task-graph-http-b"}); err != nil {
		t.Fatalf("create second graph: %v", err)
	}
	server := httptest.NewServer(Handler(k))
	defer server.Close()

	listResp, err := getWithAuth(server.URL + "/sessions/task-graph-http-a/task-graphs")
	if err != nil {
		t.Fatalf("GET task graph list: %v", err)
	}
	defer listResp.Body.Close()
	if listResp.StatusCode != http.StatusOK {
		t.Fatalf("list status = %d, want 200", listResp.StatusCode)
	}
	var graphs []TaskGraphProjection
	if err := json.NewDecoder(listResp.Body).Decode(&graphs); err != nil {
		t.Fatalf("decode graph list: %v", err)
	}
	if len(graphs) != 1 || graphs[0].GraphID != first.GraphID || graphs[0].SessionID != "task-graph-http-a" {
		t.Fatalf("graph list = %+v, want first session graph", graphs)
	}

	readResp, err := getWithAuth(server.URL + "/task-graphs/" + first.GraphID)
	if err != nil {
		t.Fatalf("GET task graph: %v", err)
	}
	defer readResp.Body.Close()
	if readResp.StatusCode != http.StatusOK {
		t.Fatalf("read status = %d, want 200", readResp.StatusCode)
	}
	var graph TaskGraphProjection
	if err := json.NewDecoder(readResp.Body).Decode(&graph); err != nil {
		t.Fatalf("decode graph: %v", err)
	}
	if graph.GraphID != first.GraphID {
		t.Fatalf("read graph = %+v, want %q", graph, first.GraphID)
	}

	missingResp, err := getWithAuth(server.URL + "/task-graphs/task_graph_missing")
	if err != nil {
		t.Fatalf("GET missing task graph: %v", err)
	}
	defer missingResp.Body.Close()
	assertErrorCode(t, missingResp, http.StatusNotFound, "not_found")
}
