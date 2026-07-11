package kernel

import (
	"errors"
	"net/http"
)

func handleGetTaskGraph(w http.ResponseWriter, r *http.Request, k *Kernel) {
	graphID := routePathValue(r, "graph_id")
	if graphID == "" {
		writeError(w, http.StatusNotFound, "not_found", "task graph route not found")
		return
	}
	graph, err := k.TaskGraph(graphID)
	if writeKernelUnavailable(w, err) {
		return
	}
	if errors.Is(err, ErrTaskGraphNotFound) {
		writeError(w, http.StatusNotFound, "not_found", "task graph not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, graph)
}

func handleListSessionTaskGraphs(w http.ResponseWriter, r *http.Request, k *Kernel) {
	sessionID := routePathValue(r, "session_id")
	if sessionID == "" {
		writeError(w, http.StatusNotFound, "not_found", "task graph route not found")
		return
	}
	graphs, err := k.TaskGraphs(sessionID)
	if writeKernelUnavailable(w, err) {
		return
	}
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, graphs)
}
