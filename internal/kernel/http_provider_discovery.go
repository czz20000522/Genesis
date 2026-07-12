package kernel

import "net/http"

func handleDiscoverProviderRouteModels(w http.ResponseWriter, r *http.Request, k *Kernel) {
	if k.providerRouteDiscoverer == nil {
		writeError(w, http.StatusServiceUnavailable, "provider_discovery_unavailable", "provider discovery is unavailable")
		return
	}
	result := k.providerRouteDiscoverer(routePathValue(r, "route_id"))
	writeJSON(w, http.StatusOK, result)
}
