package api

import (
	"encoding/json"
	"net/http"
	"sort"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/k8s-lynq/lynq/dashboard/bff/internal/kube"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// Handler contains API handlers
type Handler struct {
	kubeClient *kube.Client
	appMode    string
}

// NewHandler creates a new Handler
func NewHandler(kubeClient *kube.Client, appMode string) *Handler {
	return &Handler{
		kubeClient: kubeClient,
		appMode:    appMode,
	}
}

// writeJSON writes a JSON response
func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

// writeError writes an error response
func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

// isCRDNotFoundError checks if the error indicates CRD is not installed
func isCRDNotFoundError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "the server could not find the requested resource") ||
		strings.Contains(msg, "no matches for kind")
}

// emptyList returns an empty unstructured list
func emptyList() *unstructured.UnstructuredList {
	return &unstructured.UnstructuredList{
		Items: []unstructured.Unstructured{},
	}
}

// HealthCheck handles health check requests
func (h *Handler) HealthCheck(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// ReadyCheck handles readiness check requests
func (h *Handler) ReadyCheck(w http.ResponseWriter, r *http.Request) {
	// TODO: Check Kubernetes connectivity
	writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
}

// ListHubs lists all LynqHub resources
func (h *Handler) ListHubs(w http.ResponseWriter, r *http.Request) {
	namespace := r.URL.Query().Get("namespace")
	list, err := h.kubeClient.ListHubs(r.Context(), namespace)
	if err != nil {
		// Return empty list if CRD is not installed
		if isCRDNotFoundError(err) {
			writeJSON(w, http.StatusOK, emptyList())
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, list)
}

// GetHub gets a single LynqHub
func (h *Handler) GetHub(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	namespace := r.URL.Query().Get("namespace")

	var hub *unstructured.Unstructured
	var err error

	if namespace != "" {
		// Namespace specified - direct lookup
		hub, err = h.kubeClient.GetHub(r.Context(), name, namespace)
	} else {
		// No namespace - search all namespaces
		hub, err = h.kubeClient.FindHubByName(r.Context(), name)
	}

	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, hub)
}

// GetHubNodes gets nodes belonging to a hub
func (h *Handler) GetHubNodes(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	namespace := r.URL.Query().Get("namespace")

	nodes, err := h.kubeClient.ListNodesByHub(r.Context(), name, namespace)
	if err != nil {
		if isCRDNotFoundError(err) {
			writeJSON(w, http.StatusOK, emptyList())
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, nodes)
}

// ListForms lists all LynqForm resources
func (h *Handler) ListForms(w http.ResponseWriter, r *http.Request) {
	namespace := r.URL.Query().Get("namespace")
	list, err := h.kubeClient.ListForms(r.Context(), namespace)
	if err != nil {
		if isCRDNotFoundError(err) {
			writeJSON(w, http.StatusOK, emptyList())
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, list)
}

// GetForm gets a single LynqForm
func (h *Handler) GetForm(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	namespace := r.URL.Query().Get("namespace")

	var form *unstructured.Unstructured
	var err error

	if namespace != "" {
		// Namespace specified - direct lookup
		form, err = h.kubeClient.GetForm(r.Context(), name, namespace)
	} else {
		// No namespace - search all namespaces
		form, err = h.kubeClient.FindFormByName(r.Context(), name)
	}

	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, form)
}

// ListNodes lists all LynqNode resources
func (h *Handler) ListNodes(w http.ResponseWriter, r *http.Request) {
	namespace := r.URL.Query().Get("namespace")
	list, err := h.kubeClient.ListNodes(r.Context(), namespace)
	if err != nil {
		if isCRDNotFoundError(err) {
			writeJSON(w, http.StatusOK, emptyList())
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, list)
}

// GetNode gets a single LynqNode
func (h *Handler) GetNode(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	namespace := r.URL.Query().Get("namespace")

	var node *unstructured.Unstructured
	var err error

	if namespace != "" {
		// Namespace specified - direct lookup
		node, err = h.kubeClient.GetNode(r.Context(), name, namespace)
	} else {
		// No namespace - search all namespaces
		node, err = h.kubeClient.FindNodeByName(r.Context(), name)
	}

	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, node)
}

// GetNodeResources gets resources managed by a node
func (h *Handler) GetNodeResources(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	namespace := r.URL.Query().Get("namespace")

	ctx := r.Context()

	// Get the LynqNode
	var node *unstructured.Unstructured
	var err error

	if namespace != "" {
		node, err = h.kubeClient.GetNode(ctx, name, namespace)
	} else {
		node, err = h.kubeClient.FindNodeByName(ctx, name)
	}
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	// Get appliedResources from status
	appliedResourcesRaw, found, _ := unstructured.NestedSlice(node.Object, "status", "appliedResources")
	if !found {
		writeJSON(w, http.StatusOK, map[string]interface{}{"items": []interface{}{}})
		return
	}

	var resources []map[string]interface{}
	for _, resRaw := range appliedResourcesRaw {
		resStr, ok := resRaw.(string)
		if !ok {
			continue
		}

		parsed := parseAppliedResource(resStr)
		if parsed == nil {
			continue
		}

		// Fetch the actual resource
		resource, err := h.kubeClient.GetResource(ctx, parsed.Kind, parsed.Name, parsed.Namespace)
		if err != nil {
			// Include error info but continue
			resources = append(resources, map[string]interface{}{
				"id":        parsed.ID,
				"kind":      parsed.Kind,
				"name":      parsed.Name,
				"namespace": parsed.Namespace,
				"error":     err.Error(),
			})
			continue
		}

		// Return the resource with its ID
		resourceData := resource.Object
		resourceData["_id"] = parsed.ID
		resources = append(resources, resourceData)
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{"items": resources})
}

// GetResource gets a single Kubernetes resource
func (h *Handler) GetResource(w http.ResponseWriter, r *http.Request) {
	kind := r.URL.Query().Get("kind")
	name := r.URL.Query().Get("name")
	namespace := r.URL.Query().Get("namespace")

	if kind == "" || name == "" {
		writeError(w, http.StatusBadRequest, "kind and name are required")
		return
	}

	resource, err := h.kubeClient.GetResource(r.Context(), kind, name, namespace)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			writeError(w, http.StatusNotFound, err.Error())
		} else {
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	writeJSON(w, http.StatusOK, resource)
}

// GetFormDetails gets LynqForm with hub variables
func (h *Handler) GetFormDetails(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	namespace := r.URL.Query().Get("namespace")

	ctx := r.Context()

	// Get the LynqForm
	var form *unstructured.Unstructured
	var err error

	if namespace != "" {
		form, err = h.kubeClient.GetForm(ctx, name, namespace)
	} else {
		form, err = h.kubeClient.FindFormByName(ctx, name)
	}
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	// Get the form's namespace for hub lookup
	formNamespace := form.GetNamespace()

	// Get hubId to fetch available variables
	hubId, _, _ := unstructured.NestedString(form.Object, "spec", "hubId")

	var variables []string
	// Default variables always available
	variables = append(variables, "uid", "activate", "hubId", "templateRef")

	if hubId != "" {
		// Try to get the hub to extract variable mappings (hub is in same namespace as form)
		hub, err := h.kubeClient.GetHub(ctx, hubId, formNamespace)
		if err == nil {
			// Extract extraValueMappings
			extraMappings, found, _ := unstructured.NestedSlice(hub.Object, "spec", "extraValueMappings")
			if found {
				for _, mapping := range extraMappings {
					m, ok := mapping.(map[string]interface{})
					if ok {
						if variable, exists := m["variable"]; exists {
							variables = append(variables, "."+variable.(string))
						}
					}
				}
			}
		}
	}

	// Build response
	response := map[string]interface{}{
		"form":      form.Object,
		"variables": variables,
	}

	writeJSON(w, http.StatusOK, response)
}

// GetTopology returns topology data for visualization
func (h *Handler) GetTopology(w http.ResponseWriter, r *http.Request) {
	namespace := r.URL.Query().Get("namespace")
	ctx := r.Context()

	// Fetch all resources, returning empty lists if CRDs not installed
	hubs, err := h.kubeClient.ListHubs(ctx, namespace)
	if err != nil {
		if isCRDNotFoundError(err) {
			hubs = emptyList()
		} else {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
	}

	forms, err := h.kubeClient.ListForms(ctx, namespace)
	if err != nil {
		if isCRDNotFoundError(err) {
			forms = emptyList()
		} else {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
	}

	nodes, err := h.kubeClient.ListNodes(ctx, namespace)
	if err != nil {
		if isCRDNotFoundError(err) {
			nodes = emptyList()
		} else {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
	}

	// Fetch orphaned resources
	orphanedResources, err := h.kubeClient.ListOrphanedResources(ctx, namespace)
	if err != nil {
		// Don't fail topology if orphaned resources can't be fetched
		orphanedResources = emptyList()
	}

	// Build topology data
	topology := buildTopology(hubs, forms, nodes, orphanedResources)
	writeJSON(w, http.StatusOK, topology)
}

// ListContexts lists kubeconfig contexts (local mode only)
func (h *Handler) ListContexts(w http.ResponseWriter, r *http.Request) {
	contexts, err := h.kubeClient.ListContexts()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"items": contexts,
	})
}

// SwitchContext switches the active kubeconfig context
func (h *Handler) SwitchContext(w http.ResponseWriter, r *http.Request) {
	if h.appMode == "cluster" {
		writeError(w, http.StatusBadRequest, "context switching not supported in cluster mode")
		return
	}

	var req struct {
		Context string `json:"context"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.kubeClient.SwitchContext(req.Context); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]bool{"success": true})
}

// GetNodeEvents gets Kubernetes events for a specific LynqNode
func (h *Handler) GetNodeEvents(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	namespace := r.URL.Query().Get("namespace")

	ctx := r.Context()

	// Get the LynqNode first to validate it exists and get its namespace
	var node *unstructured.Unstructured
	var err error

	if namespace != "" {
		node, err = h.kubeClient.GetNode(ctx, name, namespace)
	} else {
		node, err = h.kubeClient.FindNodeByName(ctx, name)
	}
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	// Get the node's namespace for event lookup
	nodeNamespace := node.GetNamespace()

	h.listAndWriteEvents(w, r, nodeNamespace, name)
}

// GetEvents gets Kubernetes events for any resource by name and namespace
func (h *Handler) GetEvents(w http.ResponseWriter, r *http.Request) {
	name := r.URL.Query().Get("name")
	namespace := r.URL.Query().Get("namespace")

	if name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}

	h.listAndWriteEvents(w, r, namespace, name)
}

// listAndWriteEvents lists events and writes them to the response
func (h *Handler) listAndWriteEvents(w http.ResponseWriter, r *http.Request, namespace, involvedObjectName string) {
	ctx := r.Context()

	// List events for the object
	events, err := h.kubeClient.ListEvents(ctx, namespace, involvedObjectName)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Convert and sort events
	eventList := make([]map[string]interface{}, 0)
	for _, e := range events.Items {
		eventData := convertEventToResponse(e)
		eventList = append(eventList, eventData)
	}

	// Sort by lastTimestamp (newest first)
	sort.Slice(eventList, func(i, j int) bool {
		ti, _ := eventList[i]["lastTimestamp"].(string)
		tj, _ := eventList[j]["lastTimestamp"].(string)
		return ti > tj
	})

	writeJSON(w, http.StatusOK, map[string]interface{}{"items": eventList})
}

// convertEventToResponse converts an unstructured event to response format
func convertEventToResponse(obj unstructured.Unstructured) map[string]interface{} {
	result := make(map[string]interface{})

	result["type"], _, _ = unstructured.NestedString(obj.Object, "type")
	result["reason"], _, _ = unstructured.NestedString(obj.Object, "reason")
	result["message"], _, _ = unstructured.NestedString(obj.Object, "message")
	result["firstTimestamp"], _, _ = unstructured.NestedString(obj.Object, "firstTimestamp")
	result["lastTimestamp"], _, _ = unstructured.NestedString(obj.Object, "lastTimestamp")
	result["count"], _, _ = unstructured.NestedInt64(obj.Object, "count")

	involvedObject := make(map[string]interface{})
	involvedObject["kind"], _, _ = unstructured.NestedString(obj.Object, "involvedObject", "kind")
	involvedObject["name"], _, _ = unstructured.NestedString(obj.Object, "involvedObject", "name")
	involvedObject["namespace"], _, _ = unstructured.NestedString(obj.Object, "involvedObject", "namespace")
	result["involvedObject"] = involvedObject

	source := make(map[string]interface{})
	source["component"], _, _ = unstructured.NestedString(obj.Object, "source", "component")
	result["source"] = source

	return result
}
