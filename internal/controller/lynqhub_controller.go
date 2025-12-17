/*
Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/retry"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	lynqv1 "github.com/k8s-lynq/lynq/api/v1"
	"github.com/k8s-lynq/lynq/internal/datasource"
	"github.com/k8s-lynq/lynq/internal/metrics"
	"github.com/k8s-lynq/lynq/internal/template"
)

const (
	// Finalizer for LynqHub
	FinalizerLynqHub = "lynq.sh/hub-finalizer"
)

// LynqHubReconciler reconciles a LynqHub object
type LynqHubReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

// +kubebuilder:rbac:groups=operator.lynq.sh,resources=lynqhubs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=operator.lynq.sh,resources=lynqhubs/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=operator.lynq.sh,resources=lynqhubs/finalizers,verbs=update
// +kubebuilder:rbac:groups=operator.lynq.sh,resources=lynqnodes,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=events,verbs=create;patch

// Reconcile syncs nodes from external data source to Kubernetes
func (r *LynqHubReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Fetch LynqHub
	registry := &lynqv1.LynqHub{}
	if err := r.Get(ctx, req.NamespacedName, registry); err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		logger.Error(err, "Failed to get LynqHub")
		return ctrl.Result{}, err
	}

	// Parse syncInterval
	syncInterval, err := time.ParseDuration(registry.Spec.Source.SyncInterval)
	if err != nil {
		logger.Error(err, "Invalid syncInterval", "syncInterval", registry.Spec.Source.SyncInterval)
		syncInterval = 30 * time.Second // Default
	}

	// Handle finalizer logic
	if !registry.DeletionTimestamp.IsZero() {
		// Hub is being deleted
		if containsString(registry.Finalizers, FinalizerLynqHub) {
			// Run cleanup logic for DeletionPolicy.Retain resources
			if err := r.cleanupRetainResources(ctx, registry); err != nil {
				logger.Error(err, "Failed to cleanup retain resources")
				r.Recorder.Eventf(registry, corev1.EventTypeWarning, "CleanupFailed",
					"Failed to cleanup retain resources: %v", err)
				return ctrl.Result{RequeueAfter: 10 * time.Second}, err
			}

			// Remove finalizer
			registry.Finalizers = removeString(registry.Finalizers, FinalizerLynqHub)
			if err := r.Update(ctx, registry); err != nil {
				logger.Error(err, "Failed to remove finalizer")
				return ctrl.Result{}, err
			}
			logger.Info("Finalizer removed, registry cleanup complete")
		}
		return ctrl.Result{}, nil
	}

	// Ensure finalizer is present
	if !containsString(registry.Finalizers, FinalizerLynqHub) {
		registry.Finalizers = append(registry.Finalizers, FinalizerLynqHub)
		if err := r.Update(ctx, registry); err != nil {
			logger.Error(err, "Failed to add finalizer")
			return ctrl.Result{}, err
		}
		logger.Info("Finalizer added to registry")
		return ctrl.Result{Requeue: true}, nil
	}

	// Get all templates that reference this registry
	templates, err := r.getTemplatesForRegistry(ctx, registry)
	if err != nil {
		logger.Error(err, "Failed to get templates for registry")
		r.updateStatus(ctx, registry, 0, 0, 0, 0, false)
		return ctrl.Result{RequeueAfter: syncInterval}, err
	}

	// Connect to database and query nodes
	nodeRows, err := r.queryDatabase(ctx, registry)
	if err != nil {
		logger.Error(err, "Failed to query database")
		r.Recorder.Eventf(registry, corev1.EventTypeWarning, "DatabaseQueryFailed",
			"Failed to query database: %v", err)
		r.updateStatus(ctx, registry, int32(len(templates)), 0, 0, 0, false)
		return ctrl.Result{RequeueAfter: syncInterval}, err
	}

	// Get existing LynqNode CRs
	existingNodes, err := r.getExistingLynqNodes(ctx, registry)
	if err != nil {
		logger.Error(err, "Failed to list existing nodes")
		return ctrl.Result{RequeueAfter: syncInterval}, err
	}

	// Build desired node set: key = {template-name}-{uid}
	type NodeKey struct {
		TemplateName string
		UID          string
	}
	desired := make(map[NodeKey]struct {
		Template *lynqv1.LynqForm
		Row      datasource.NodeRow
	})

	for _, tmpl := range templates {
		for _, row := range nodeRows {
			key := NodeKey{
				TemplateName: tmpl.Name,
				UID:          row.UID,
			}
			desired[key] = struct {
				Template *lynqv1.LynqForm
				Row      datasource.NodeRow
			}{
				Template: tmpl,
				Row:      row,
			}
		}
	}

	// Build existing node map: key = {template-name}-{uid}
	existing := make(map[NodeKey]*lynqv1.LynqNode)
	for i := range existingNodes.Items {
		node := &existingNodes.Items[i]
		key := NodeKey{
			TemplateName: node.Spec.TemplateRef,
			UID:          node.Spec.UID,
		}
		existing[key] = node
	}

	// Group existing nodes by template for maxSkew checking
	nodesByTemplate := make(map[string][]*lynqv1.LynqNode)
	for key, node := range existing {
		nodesByTemplate[key.TemplateName] = append(nodesByTemplate[key.TemplateName], node)
	}

	// Track throttled updates for events
	throttledByTemplate := make(map[string]int)

	// Track nodes updated in THIS reconcile iteration per template
	// This is critical for maxSkew enforcement because templateNodes snapshot doesn't reflect
	// updates made within this loop iteration
	updatedInThisIteration := make(map[string]int32)

	// Create/update nodes for each template-row combination
	for key, desired := range desired {
		tmpl := desired.Template
		templateNodes := nodesByTemplate[tmpl.Name]

		if existingLynqNode, exists := existing[key]; !exists {
			// Create new LynqNode - check maxSkew before creating
			if r.canUpdateNodeWithCount(ctx, tmpl, templateNodes, updatedInThisIteration[tmpl.Name]) {
				if err := r.createLynqNode(ctx, registry, tmpl, desired.Row); err != nil {
					// Ignore AlreadyExists errors (can happen due to concurrent reconciliations)
					if !errors.IsAlreadyExists(err) {
						logger.Error(err, "Failed to create LynqNode", "template", key.TemplateName, "uid", key.UID)
					}
				} else {
					// Successfully created - track it
					updatedInThisIteration[tmpl.Name]++
				}
			} else {
				// Throttled by maxSkew
				throttledByTemplate[tmpl.Name]++
			}
		} else {
			// Update existing LynqNode if data or template changed
			if r.shouldUpdateLynqNode(ctx, registry, existingLynqNode, desired.Row) {
				// Check maxSkew before updating
				if r.canUpdateNodeWithCount(ctx, tmpl, templateNodes, updatedInThisIteration[tmpl.Name]) {
					if err := r.updateLynqNode(ctx, registry, tmpl, existingLynqNode, desired.Row); err != nil {
						logger.Error(err, "Failed to update LynqNode", "template", key.TemplateName, "uid", key.UID)
					} else {
						// Successfully updated - track it
						updatedInThisIteration[tmpl.Name]++
					}
				} else {
					// Throttled by maxSkew
					throttledByTemplate[tmpl.Name]++
				}
			}
		}
	}

	// Emit events for throttled updates
	for tmplName, throttledCount := range throttledByTemplate {
		// Find the template to get maxSkew value
		for _, tmpl := range templates {
			if tmpl.Name == tmplName && tmpl.Spec.Rollout != nil {
				r.Recorder.Eventf(registry, corev1.EventTypeNormal, "RolloutThrottled",
					"LynqForm '%s': %d node updates throttled (maxSkew=%d, currently updating=%d)",
					tmplName, throttledCount, tmpl.Spec.Rollout.MaxSkew,
					r.countUpdatingNodes(ctx, nodesByTemplate[tmplName], tmpl.Generation))
				break
			}
		}
	}

	// Delete nodes no longer in desired set
	// This handles:
	// 1. Rows deleted from database
	// 2. Rows with activate=false
	// 3. Templates deleted/changed
	deletedCount := 0
	for key, node := range existing {
		if _, stillExists := desired[key]; !stillExists {
			logger.Info("Deleting LynqNode (no longer in desired set)",
				"node", node.Name,
				"template", key.TemplateName,
				"uid", key.UID,
				"reason", "row removed from database or activate=false or template changed")

			// Emit detailed deletion event
			r.Recorder.Eventf(registry, corev1.EventTypeNormal, "NodeDeleting",
				"Deleting LynqNode '%s' (template: %s, uid: %s) - no longer in active dataset. "+
					"This could be due to: row deletion, activate=false, or template change.",
				node.Name, key.TemplateName, key.UID)

			if err := r.Delete(ctx, node); err != nil {
				if !errors.IsNotFound(err) {
					logger.Error(err, "Failed to delete LynqNode", "node", node.Name, "template", key.TemplateName, "uid", key.UID)
					r.Recorder.Eventf(registry, corev1.EventTypeWarning, "NodeDeletionFailed",
						"Failed to delete LynqNode '%s': %v", node.Name, err)
				}
			} else {
				deletedCount++
				r.Recorder.Eventf(registry, corev1.EventTypeNormal, "NodeDeleted",
					"Successfully deleted LynqNode '%s' (template: %s, uid: %s)",
					node.Name, key.TemplateName, key.UID)
			}
		}
	}

	if deletedCount > 0 {
		logger.Info("Garbage collection completed", "deletedNodes", deletedCount)
	}

	// Update status
	readyCount, failedCount := r.countLynqNodeStatus(ctx, registry)
	totalDesired := int32(len(templates)) * int32(len(nodeRows))
	r.updateStatus(ctx, registry, int32(len(templates)), totalDesired, readyCount, failedCount, true)

	return ctrl.Result{RequeueAfter: syncInterval}, nil
}

// queryDatabase connects to database and retrieves node rows
func (r *LynqHubReconciler) queryDatabase(ctx context.Context, registry *lynqv1.LynqHub) ([]datasource.NodeRow, error) {
	// Determine datasource type
	sourceType := datasource.SourceType(registry.Spec.Source.Type)

	// Get password from Secret (MySQL/PostgreSQL specific)
	password := ""
	if registry.Spec.Source.MySQL != nil && registry.Spec.Source.MySQL.PasswordRef != nil {
		secret := &corev1.Secret{}
		if err := r.Get(ctx, types.NamespacedName{
			Name:      registry.Spec.Source.MySQL.PasswordRef.Name,
			Namespace: registry.Namespace,
		}, secret); err != nil {
			return nil, fmt.Errorf("failed to get password secret: %w", err)
		}
		password = string(secret.Data[registry.Spec.Source.MySQL.PasswordRef.Key])
	}

	// Build datasource config
	config, table, err := r.buildDatasourceConfig(registry, password)
	if err != nil {
		return nil, err
	}

	// Create datasource adapter
	ds, err := datasource.NewDatasource(sourceType, config)
	if err != nil {
		return nil, fmt.Errorf("failed to create datasource: %w", err)
	}
	defer func() {
		_ = ds.Close() // Best effort close
	}()

	// Query nodes
	queryConfig := datasource.QueryConfig{
		Table: table,
		ValueMappings: datasource.ValueMappings{
			UID:       registry.Spec.ValueMappings.UID,
			HostOrURL: registry.Spec.ValueMappings.HostOrURL,
			Activate:  registry.Spec.ValueMappings.Activate,
		},
		ExtraMappings: registry.Spec.ExtraValueMappings,
	}

	return ds.QueryNodes(ctx, queryConfig)
}

// buildDatasourceConfig builds datasource configuration from LynqHub spec
func (r *LynqHubReconciler) buildDatasourceConfig(registry *lynqv1.LynqHub, password string) (datasource.Config, string, error) {
	switch registry.Spec.Source.Type {
	case lynqv1.SourceTypeMySQL:
		mysql := registry.Spec.Source.MySQL
		if mysql == nil {
			return datasource.Config{}, "", fmt.Errorf("MySQL configuration is nil")
		}

		config := datasource.Config{
			Host:     mysql.Host,
			Port:     mysql.Port,
			Username: mysql.Username,
			Password: password,
			Database: mysql.Database,
		}

		return config, mysql.Table, nil

	default:
		return datasource.Config{}, "", fmt.Errorf("unsupported source type: %s", registry.Spec.Source.Type)
	}
}

// getTemplatesForRegistry retrieves all LynqForms that reference this registry
func (r *LynqHubReconciler) getTemplatesForRegistry(ctx context.Context, registry *lynqv1.LynqHub) ([]*lynqv1.LynqForm, error) {
	// List all templates in the same namespace
	templateList := &lynqv1.LynqFormList{}
	if err := r.List(ctx, templateList, client.InNamespace(registry.Namespace)); err != nil {
		return nil, fmt.Errorf("failed to list templates: %w", err)
	}

	// Find all templates with matching hubId
	var templates []*lynqv1.LynqForm
	for i := range templateList.Items {
		tmpl := &templateList.Items[i]
		if tmpl.Spec.HubID == registry.Name {
			templates = append(templates, tmpl)
		}
	}

	return templates, nil
}

// renderAllTemplateResources renders all resources from a template with the given variables
func (r *LynqHubReconciler) renderAllTemplateResources(
	tmpl *lynqv1.LynqForm,
	vars template.Variables,
) (*lynqv1.LynqNodeSpec, error) {
	engine := template.NewEngine()

	spec := &lynqv1.LynqNodeSpec{
		ServiceAccounts:          make([]lynqv1.TResource, 0),
		Deployments:              make([]lynqv1.TResource, 0),
		StatefulSets:             make([]lynqv1.TResource, 0),
		Services:                 make([]lynqv1.TResource, 0),
		Ingresses:                make([]lynqv1.TResource, 0),
		ConfigMaps:               make([]lynqv1.TResource, 0),
		Secrets:                  make([]lynqv1.TResource, 0),
		PersistentVolumeClaims:   make([]lynqv1.TResource, 0),
		Jobs:                     make([]lynqv1.TResource, 0),
		CronJobs:                 make([]lynqv1.TResource, 0),
		PodDisruptionBudgets:     make([]lynqv1.TResource, 0),
		NetworkPolicies:          make([]lynqv1.TResource, 0),
		HorizontalPodAutoscalers: make([]lynqv1.TResource, 0),
		Namespaces:               make([]lynqv1.TResource, 0),
		Manifests:                make([]lynqv1.TResource, 0),
	}

	// Render each resource type
	var err error

	spec.ServiceAccounts, err = r.renderResourceList(engine, tmpl.Spec.ServiceAccounts, vars)
	if err != nil {
		return nil, fmt.Errorf("failed to render serviceAccounts: %w", err)
	}

	spec.Deployments, err = r.renderResourceList(engine, tmpl.Spec.Deployments, vars)
	if err != nil {
		return nil, fmt.Errorf("failed to render deployments: %w", err)
	}

	spec.StatefulSets, err = r.renderResourceList(engine, tmpl.Spec.StatefulSets, vars)
	if err != nil {
		return nil, fmt.Errorf("failed to render statefulSets: %w", err)
	}

	spec.DaemonSets, err = r.renderResourceList(engine, tmpl.Spec.DaemonSets, vars)
	if err != nil {
		return nil, fmt.Errorf("failed to render daemonSets: %w", err)
	}

	spec.Services, err = r.renderResourceList(engine, tmpl.Spec.Services, vars)
	if err != nil {
		return nil, fmt.Errorf("failed to render services: %w", err)
	}

	spec.Ingresses, err = r.renderResourceList(engine, tmpl.Spec.Ingresses, vars)
	if err != nil {
		return nil, fmt.Errorf("failed to render ingresses: %w", err)
	}

	spec.ConfigMaps, err = r.renderResourceList(engine, tmpl.Spec.ConfigMaps, vars)
	if err != nil {
		return nil, fmt.Errorf("failed to render configMaps: %w", err)
	}

	spec.Secrets, err = r.renderResourceList(engine, tmpl.Spec.Secrets, vars)
	if err != nil {
		return nil, fmt.Errorf("failed to render secrets: %w", err)
	}

	spec.PersistentVolumeClaims, err = r.renderResourceList(engine, tmpl.Spec.PersistentVolumeClaims, vars)
	if err != nil {
		return nil, fmt.Errorf("failed to render persistentVolumeClaims: %w", err)
	}

	spec.Jobs, err = r.renderResourceList(engine, tmpl.Spec.Jobs, vars)
	if err != nil {
		return nil, fmt.Errorf("failed to render jobs: %w", err)
	}

	spec.CronJobs, err = r.renderResourceList(engine, tmpl.Spec.CronJobs, vars)
	if err != nil {
		return nil, fmt.Errorf("failed to render cronJobs: %w", err)
	}

	spec.PodDisruptionBudgets, err = r.renderResourceList(engine, tmpl.Spec.PodDisruptionBudgets, vars)
	if err != nil {
		return nil, fmt.Errorf("failed to render podDisruptionBudgets: %w", err)
	}

	spec.NetworkPolicies, err = r.renderResourceList(engine, tmpl.Spec.NetworkPolicies, vars)
	if err != nil {
		return nil, fmt.Errorf("failed to render networkPolicies: %w", err)
	}

	spec.HorizontalPodAutoscalers, err = r.renderResourceList(engine, tmpl.Spec.HorizontalPodAutoscalers, vars)
	if err != nil {
		return nil, fmt.Errorf("failed to render horizontalPodAutoscalers: %w", err)
	}

	spec.Namespaces, err = r.renderResourceList(engine, tmpl.Spec.Namespaces, vars)
	if err != nil {
		return nil, fmt.Errorf("failed to render namespaces: %w", err)
	}

	spec.Manifests, err = r.renderResourceList(engine, tmpl.Spec.Manifests, vars)
	if err != nil {
		return nil, fmt.Errorf("failed to render manifests: %w", err)
	}

	return spec, nil
}

// renderResourceList renders a list of template resources
func (r *LynqHubReconciler) renderResourceList(
	engine *template.Engine,
	resources []lynqv1.TResource,
	vars template.Variables,
) ([]lynqv1.TResource, error) {
	if len(resources) == 0 {
		return []lynqv1.TResource{}, nil
	}

	rendered := make([]lynqv1.TResource, len(resources))
	for i, resource := range resources {
		var err error
		rendered[i], err = r.renderResource(engine, resource, vars)
		if err != nil {
			return nil, fmt.Errorf("failed to render resource %s: %w", resource.ID, err)
		}
	}

	return rendered, nil
}

// renderResource renders a single template resource
func (r *LynqHubReconciler) renderResource(
	engine *template.Engine,
	resource lynqv1.TResource,
	vars template.Variables,
) (lynqv1.TResource, error) {
	rendered := resource

	// Render name template
	if resource.NameTemplate != "" {
		name, err := engine.Render(resource.NameTemplate, vars)
		if err != nil {
			return resource, fmt.Errorf("failed to render name template: %w", err)
		}
		rendered.NameTemplate = name
	}

	// Render labels template
	if len(resource.LabelsTemplate) > 0 {
		labels, err := engine.RenderMap(resource.LabelsTemplate, vars)
		if err != nil {
			return resource, fmt.Errorf("failed to render labels template: %w", err)
		}
		rendered.LabelsTemplate = labels
	}

	// Render annotations template
	if len(resource.AnnotationsTemplate) > 0 {
		annotations, err := engine.RenderMap(resource.AnnotationsTemplate, vars)
		if err != nil {
			return resource, fmt.Errorf("failed to render annotations template: %w", err)
		}
		rendered.AnnotationsTemplate = annotations
	}

	// Note: resource.Spec (unstructured.Unstructured) will be rendered by LynqNode controller
	// when it actually applies the resources, as it needs deep recursive rendering

	return rendered, nil
}

// createLynqNode creates a new LynqNode CR
func (r *LynqHubReconciler) createLynqNode(ctx context.Context, registry *lynqv1.LynqHub, tmpl *lynqv1.LynqForm, row datasource.NodeRow) error {
	logger := log.FromContext(ctx)

	// 1. Build template variables
	vars := template.BuildVariables(row.UID, row.HostOrURL, row.Activate, row.Extra)
	// Add hubId and templateRef for template rendering (needed for labelsTemplate, annotationsTemplate)
	vars["hubId"] = registry.Name
	vars["templateRef"] = tmpl.Name

	// 2. Render all template resources
	renderedSpec, err := r.renderAllTemplateResources(tmpl, vars)
	if err != nil {
		logger.Error(err, "Failed to render template resources", "node", row.UID)
		r.Recorder.Eventf(registry, corev1.EventTypeWarning, "TemplateRenderFailed",
			"Failed to render template for node %s: %v", row.UID, err)
		return fmt.Errorf("failed to render template: %w", err)
	}

	// 4. Marshal extra values to JSON for annotation
	extraJSON, err := json.Marshal(row.Extra)
	if err != nil {
		logger.Error(err, "Failed to marshal extra values", "node", row.UID)
		extraJSON = []byte("{}")
	}

	// 3. Create LynqNode CR with rendered resources
	// Name format: {uid}-{template-name} to support multiple templates per registry
	node := &lynqv1.LynqNode{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-%s", row.UID, tmpl.Name),
			Namespace: registry.Namespace,
			Labels: map[string]string{
				"lynq.sh/hub": registry.Name,
				"lynq.sh/uid": row.UID,
			},
			Annotations: map[string]string{
				"lynq.sh/hostOrUrl":                     row.HostOrURL,
				"lynq.sh/activate":                      row.Activate,
				"lynq.sh/extra":                         string(extraJSON),
				lynqv1.AnnotationTemplateGeneration:     fmt.Sprintf("%d", tmpl.Generation),
				"lynq.sh/hubId":                         registry.Name,
				lynqv1.AnnotationRolloutUpdateStartTime: time.Now().Format(time.RFC3339),
			},
		},
		Spec: *renderedSpec,
	}

	// Set UID and TemplateRef
	node.Spec.UID = row.UID
	node.Spec.TemplateRef = tmpl.Name

	// Set owner reference
	if err := ctrl.SetControllerReference(registry, node, r.Scheme); err != nil {
		return fmt.Errorf("failed to set owner reference: %w", err)
	}

	logger.Info("Creating LynqNode", "node", node.Name, "uid", row.UID, "template", tmpl.Name)
	return r.Create(ctx, node)
}

// shouldUpdateLynqNode checks if a node needs to be updated based on data changes or template changes
func (r *LynqHubReconciler) shouldUpdateLynqNode(ctx context.Context, registry *lynqv1.LynqHub, node *lynqv1.LynqNode, row datasource.NodeRow) bool {
	// Check if stored data differs from current row data
	storedHostOrURL := node.Annotations["lynq.sh/hostOrUrl"]
	storedActivate := node.Annotations["lynq.sh/activate"]
	storedExtraJSON := node.Annotations["lynq.sh/extra"]

	if storedHostOrURL != row.HostOrURL || storedActivate != row.Activate {
		return true
	}

	// Compare extra values
	currentExtraJSON, err := json.Marshal(row.Extra)
	if err != nil {
		// If can't marshal, assume changed
		return true
	}

	if storedExtraJSON != string(currentExtraJSON) {
		return true
	}

	// Check if template has been updated
	// Use node.Spec.TemplateRef to find the correct template for this node
	tmpl := &lynqv1.LynqForm{}
	if err := r.Get(ctx, types.NamespacedName{
		Name:      node.Spec.TemplateRef,
		Namespace: registry.Namespace,
	}, tmpl); err != nil {
		// If we can't get the template, assume update is needed
		return true
	}

	// Compare template generation
	storedTemplateGeneration := node.Annotations["lynq.sh/template-generation"]
	currentTemplateGeneration := fmt.Sprintf("%d", tmpl.Generation)

	return storedTemplateGeneration != currentTemplateGeneration
}

// updateLynqNode updates an existing LynqNode CR with new data from database
func (r *LynqHubReconciler) updateLynqNode(ctx context.Context, registry *lynqv1.LynqHub, tmpl *lynqv1.LynqForm, node *lynqv1.LynqNode, row datasource.NodeRow) error {
	logger := log.FromContext(ctx)

	// Check what triggered the update
	oldTemplateGeneration := node.Annotations["lynq.sh/template-generation"]
	newTemplateGeneration := fmt.Sprintf("%d", tmpl.Generation)
	templateChanged := oldTemplateGeneration != newTemplateGeneration
	dataChanged := node.Annotations["lynq.sh/hostOrUrl"] != row.HostOrURL ||
		node.Annotations["lynq.sh/activate"] != row.Activate

	// 1. Build template variables with new data
	vars := template.BuildVariables(row.UID, row.HostOrURL, row.Activate, row.Extra)
	// Add hubId and templateRef for template rendering (needed for labelsTemplate, annotationsTemplate)
	vars["hubId"] = registry.Name
	vars["templateRef"] = tmpl.Name

	// 2. Render all template resources
	renderedSpec, err := r.renderAllTemplateResources(tmpl, vars)
	if err != nil {
		logger.Error(err, "Failed to render template resources", "node", row.UID)
		r.Recorder.Eventf(registry, corev1.EventTypeWarning, "TemplateRenderFailed",
			"Failed to render template for node %s: %v", row.UID, err)
		return fmt.Errorf("failed to render template: %w", err)
	}

	// 4. Marshal extra values to JSON for annotation
	extraJSON, err := json.Marshal(row.Extra)
	if err != nil {
		logger.Error(err, "Failed to marshal extra values", "node", row.UID)
		extraJSON = []byte("{}")
	}

	// 5. Count resources by type for detailed event
	resourceCounts := r.countResourcesByType(renderedSpec)
	totalResources := resourceCounts["total"]

	// 6. Retry update on conflict (handles concurrent modifications by LynqNode controller)
	if err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		// Get the latest version of the LynqNode
		latest := &lynqv1.LynqNode{}
		if err := r.Get(ctx, client.ObjectKeyFromObject(node), latest); err != nil {
			return err
		}

		// Update LynqNode annotations and spec with latest version
		if latest.Annotations == nil {
			latest.Annotations = make(map[string]string)
		}
		latest.Annotations["lynq.sh/hostOrUrl"] = row.HostOrURL
		latest.Annotations["lynq.sh/activate"] = row.Activate
		latest.Annotations["lynq.sh/extra"] = string(extraJSON)
		latest.Annotations[lynqv1.AnnotationTemplateGeneration] = newTemplateGeneration
		latest.Annotations["lynq.sh/hubId"] = registry.Name
		// Update rollout start time for progress deadline tracking
		latest.Annotations[lynqv1.AnnotationRolloutUpdateStartTime] = time.Now().Format(time.RFC3339)

		// Update spec with newly rendered resources
		latest.Spec = *renderedSpec
		latest.Spec.UID = row.UID
		latest.Spec.TemplateRef = tmpl.Name

		// Perform the update with the latest version
		return r.Update(ctx, latest)
	}); err != nil {
		return err
	}

	// 7. Emit detailed events based on what changed (only after successful update)
	if templateChanged {
		// Template changed - emit detailed Applied event
		resourceDetails := r.formatResourceDetails(resourceCounts)
		r.Recorder.Eventf(node, corev1.EventTypeNormal, "TemplateApplied",
			"Applying LynqForm '%s' changes (generation: %s -> %s). "+
				"Total %d resources will be reconciled: %s. "+
				"Hub: %s, UID: %s",
			tmpl.Name, oldTemplateGeneration, newTemplateGeneration,
			totalResources, resourceDetails,
			registry.Name, row.UID)

		logger.V(1).Info("Applying template changes to LynqNode",
			"node", node.Name,
			"template", tmpl.Name,
			"oldGeneration", oldTemplateGeneration,
			"newGeneration", newTemplateGeneration,
			"totalResources", totalResources)
	} else if dataChanged {
		// Only data changed
		r.Recorder.Eventf(node, corev1.EventTypeNormal, "DataUpdated",
			"LynqNode data updated from registry '%s'. "+
				"Total %d resources may be affected. "+
				"Template: %s (generation: %s)",
			registry.Name, totalResources,
			tmpl.Name, newTemplateGeneration)

		logger.V(1).Info("Updating LynqNode data",
			"node", node.Name,
			"template", tmpl.Name,
			"totalResources", totalResources)
	}
	// Note: Extra value changes don't need an event - the update is sufficient
	// and logging at DEBUG level to reduce noise

	return nil
}

// countResourcesByType counts resources by type in a LynqNodeSpec
func (r *LynqHubReconciler) countResourcesByType(spec *lynqv1.LynqNodeSpec) map[string]int {
	counts := make(map[string]int)
	total := 0

	if len(spec.ServiceAccounts) > 0 {
		counts["ServiceAccounts"] = len(spec.ServiceAccounts)
		total += len(spec.ServiceAccounts)
	}
	if len(spec.Deployments) > 0 {
		counts["Deployments"] = len(spec.Deployments)
		total += len(spec.Deployments)
	}
	if len(spec.StatefulSets) > 0 {
		counts["StatefulSets"] = len(spec.StatefulSets)
		total += len(spec.StatefulSets)
	}
	if len(spec.Services) > 0 {
		counts["Services"] = len(spec.Services)
		total += len(spec.Services)
	}
	if len(spec.Ingresses) > 0 {
		counts["Ingresses"] = len(spec.Ingresses)
		total += len(spec.Ingresses)
	}
	if len(spec.ConfigMaps) > 0 {
		counts["ConfigMaps"] = len(spec.ConfigMaps)
		total += len(spec.ConfigMaps)
	}
	if len(spec.Secrets) > 0 {
		counts["Secrets"] = len(spec.Secrets)
		total += len(spec.Secrets)
	}
	if len(spec.PersistentVolumeClaims) > 0 {
		counts["PVCs"] = len(spec.PersistentVolumeClaims)
		total += len(spec.PersistentVolumeClaims)
	}
	if len(spec.Jobs) > 0 {
		counts["Jobs"] = len(spec.Jobs)
		total += len(spec.Jobs)
	}
	if len(spec.CronJobs) > 0 {
		counts["CronJobs"] = len(spec.CronJobs)
		total += len(spec.CronJobs)
	}
	if len(spec.PodDisruptionBudgets) > 0 {
		counts["PodDisruptionBudgets"] = len(spec.PodDisruptionBudgets)
		total += len(spec.PodDisruptionBudgets)
	}
	if len(spec.NetworkPolicies) > 0 {
		counts["NetworkPolicies"] = len(spec.NetworkPolicies)
		total += len(spec.NetworkPolicies)
	}
	if len(spec.HorizontalPodAutoscalers) > 0 {
		counts["HorizontalPodAutoscalers"] = len(spec.HorizontalPodAutoscalers)
		total += len(spec.HorizontalPodAutoscalers)
	}
	if len(spec.Manifests) > 0 {
		counts["Manifests"] = len(spec.Manifests)
		total += len(spec.Manifests)
	}

	counts["total"] = total
	return counts
}

// formatResourceDetails formats resource counts into a readable string
func (r *LynqHubReconciler) formatResourceDetails(counts map[string]int) string {
	if counts["total"] == 0 {
		return NoResourcesMessage
	}

	var details []string
	if count, ok := counts["ServiceAccounts"]; ok {
		details = append(details, fmt.Sprintf("%d ServiceAccount(s)", count))
	}
	if count, ok := counts["Deployments"]; ok {
		details = append(details, fmt.Sprintf("%d Deployment(s)", count))
	}
	if count, ok := counts["StatefulSets"]; ok {
		details = append(details, fmt.Sprintf("%d StatefulSet(s)", count))
	}
	if count, ok := counts["Services"]; ok {
		details = append(details, fmt.Sprintf("%d Service(s)", count))
	}
	if count, ok := counts["Ingresses"]; ok {
		details = append(details, fmt.Sprintf("%d Ingress(es)", count))
	}
	if count, ok := counts["ConfigMaps"]; ok {
		details = append(details, fmt.Sprintf("%d ConfigMap(s)", count))
	}
	if count, ok := counts["Secrets"]; ok {
		details = append(details, fmt.Sprintf("%d Secret(s)", count))
	}
	if count, ok := counts["PVCs"]; ok {
		details = append(details, fmt.Sprintf("%d PVC(s)", count))
	}
	if count, ok := counts["Jobs"]; ok {
		details = append(details, fmt.Sprintf("%d Job(s)", count))
	}
	if count, ok := counts["CronJobs"]; ok {
		details = append(details, fmt.Sprintf("%d CronJob(s)", count))
	}
	if count, ok := counts["PodDisruptionBudgets"]; ok {
		details = append(details, fmt.Sprintf("%d PodDisruptionBudget(s)", count))
	}
	if count, ok := counts["NetworkPolicies"]; ok {
		details = append(details, fmt.Sprintf("%d NetworkPolicy(ies)", count))
	}
	if count, ok := counts["HorizontalPodAutoscalers"]; ok {
		details = append(details, fmt.Sprintf("%d HorizontalPodAutoscaler(s)", count))
	}
	if count, ok := counts["Manifests"]; ok {
		details = append(details, fmt.Sprintf("%d Manifest(s)", count))
	}

	if len(details) == 0 {
		return NoResourcesMessage
	}

	return fmt.Sprintf("%s", details[0:])
}

// getExistingLynqNodes lists LynqNode CRs managed by this registry
func (r *LynqHubReconciler) getExistingLynqNodes(ctx context.Context, registry *lynqv1.LynqHub) (*lynqv1.LynqNodeList, error) {
	nodeList := &lynqv1.LynqNodeList{}
	if err := r.List(ctx, nodeList, client.InNamespace(registry.Namespace), client.MatchingLabels{
		"lynq.sh/hub": registry.Name,
	}); err != nil {
		return nil, err
	}
	return nodeList, nil
}

// countLynqNodeStatus counts ready and failed nodes
func (r *LynqHubReconciler) countLynqNodeStatus(ctx context.Context, registry *lynqv1.LynqHub) (int32, int32) {
	nodes, err := r.getExistingLynqNodes(ctx, registry)
	if err != nil {
		return 0, 0
	}

	var ready, failed int32
	for _, node := range nodes.Items {
		for _, cond := range node.Status.Conditions {
			if cond.Type == "Ready" {
				if cond.Status == metav1.ConditionTrue {
					ready++
				} else {
					failed++
				}
				break
			}
		}
	}

	return ready, failed
}

// updateStatus updates LynqHub status with retry on conflict
func (r *LynqHubReconciler) updateStatus(ctx context.Context, registry *lynqv1.LynqHub, referencingTemplates, desired, ready, failed int32, synced bool) {
	logger := log.FromContext(ctx)

	// Record metrics first (these don't depend on the status update)
	metrics.HubDesired.WithLabelValues(registry.Name, registry.Namespace).Set(float64(desired))
	metrics.HubReady.WithLabelValues(registry.Name, registry.Namespace).Set(float64(ready))
	metrics.HubFailed.WithLabelValues(registry.Name, registry.Namespace).Set(float64(failed))

	// Retry status update on conflict
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		// Get the latest version of the registry
		key := client.ObjectKeyFromObject(registry)
		latest := &lynqv1.LynqHub{}
		if err := r.Get(ctx, key, latest); err != nil {
			return err
		}

		// Update status fields
		latest.Status.ReferencingTemplates = referencingTemplates
		latest.Status.Desired = desired
		latest.Status.Ready = ready
		latest.Status.Failed = failed
		latest.Status.ObservedGeneration = latest.Generation

		// Prepare condition
		condition := metav1.Condition{
			Type:               "Ready",
			Status:             metav1.ConditionTrue,
			Reason:             "DatabaseConnected",
			Message:            "Successfully connected to database and queried node data",
			LastTransitionTime: metav1.Now(),
		}
		if !synced {
			condition.Status = metav1.ConditionFalse
			condition.Reason = "DatabaseConnectionFailed"
			condition.Message = "Failed to connect to database or query node data"
		}

		// Update or append condition
		found := false
		for i := range latest.Status.Conditions {
			if latest.Status.Conditions[i].Type == condition.Type {
				latest.Status.Conditions[i] = condition
				found = true
				break
			}
		}
		if !found {
			latest.Status.Conditions = append(latest.Status.Conditions, condition)
		}

		// Update status subresource
		return r.Status().Update(ctx, latest)
	})

	if err != nil {
		logger.Error(err, "Failed to update LynqHub status after retries")
	}
}

// cleanupRetainResources handles DeletionPolicy.Retain resources when Hub is deleted
func (r *LynqHubReconciler) cleanupRetainResources(ctx context.Context, registry *lynqv1.LynqHub) error {
	logger := log.FromContext(ctx)

	// Get all nodes managed by this registry
	nodes, err := r.getExistingLynqNodes(ctx, registry)
	if err != nil {
		return fmt.Errorf("failed to list nodes: %w", err)
	}

	logger.Info("Cleaning up retain resources", "hub", registry.Name, "nodeCount", len(nodes.Items))

	// For each node, process Retain resources
	for _, node := range nodes.Items {
		if err := r.processRetainResourcesForLynqNode(ctx, &node); err != nil {
			logger.Error(err, "Failed to process retain resources for node", "node", node.Name)
			// Continue with other nodes even if one fails
			r.Recorder.Eventf(registry, corev1.EventTypeWarning, "RetainResourceProcessFailed",
				"Failed to process retain resources for node %s: %v", node.Name, err)
		}
	}

	logger.Info("Cleanup complete for retain resources", "hub", registry.Name)
	return nil
}

// processRetainResourcesForLynqNode removes ownerReferences from Retain resources
//
//nolint:unparam // error return kept for future resource cleanup error handling
func (r *LynqHubReconciler) processRetainResourcesForLynqNode(ctx context.Context, node *lynqv1.LynqNode) error {
	logger := log.FromContext(ctx)

	// Collect all resources with DeletionPolicy.Retain
	allResources := []lynqv1.TResource{}
	allResources = append(allResources, node.Spec.ServiceAccounts...)
	allResources = append(allResources, node.Spec.Deployments...)
	allResources = append(allResources, node.Spec.StatefulSets...)
	allResources = append(allResources, node.Spec.DaemonSets...)
	allResources = append(allResources, node.Spec.Services...)
	allResources = append(allResources, node.Spec.Ingresses...)
	allResources = append(allResources, node.Spec.ConfigMaps...)
	allResources = append(allResources, node.Spec.Secrets...)
	allResources = append(allResources, node.Spec.PersistentVolumeClaims...)
	allResources = append(allResources, node.Spec.Jobs...)
	allResources = append(allResources, node.Spec.CronJobs...)
	allResources = append(allResources, node.Spec.PodDisruptionBudgets...)
	allResources = append(allResources, node.Spec.NetworkPolicies...)
	allResources = append(allResources, node.Spec.HorizontalPodAutoscalers...)
	allResources = append(allResources, node.Spec.Namespaces...)
	allResources = append(allResources, node.Spec.Manifests...)

	// Process each resource with Retain policy
	for _, resource := range allResources {
		if resource.DeletionPolicy != lynqv1.DeletionPolicyRetain {
			continue
		}

		logger.Info("Processing retain resource", "node", node.Name, "resourceId", resource.ID, "name", resource.NameTemplate)

		// Get the resource from cluster
		obj := resource.Spec.DeepCopy()
		obj.SetName(resource.NameTemplate)
		// All resources are in the same namespace as the LynqNode CR
		obj.SetNamespace(node.Namespace)

		key := client.ObjectKey{
			Name:      obj.GetName(),
			Namespace: obj.GetNamespace(),
		}

		if err := r.Get(ctx, key, obj); err != nil {
			if errors.IsNotFound(err) {
				logger.Info("Retain resource not found, skipping", "name", key.Name, "namespace", key.Namespace)
				continue
			}
			logger.Error(err, "Failed to get retain resource", "name", key.Name, "namespace", key.Namespace)
			continue
		}

		// Remove ownerReferences that point to this node
		ownerRefs := obj.GetOwnerReferences()
		newOwnerRefs := []metav1.OwnerReference{}
		for _, ref := range ownerRefs {
			if ref.UID != node.UID {
				newOwnerRefs = append(newOwnerRefs, ref)
			}
		}

		if len(newOwnerRefs) != len(ownerRefs) {
			obj.SetOwnerReferences(newOwnerRefs)
			if err := r.Update(ctx, obj); err != nil {
				logger.Error(err, "Failed to remove ownerReference from retain resource", "name", key.Name, "namespace", key.Namespace)
				continue
			}
			logger.Info("Removed ownerReference from retain resource", "name", key.Name, "namespace", key.Namespace)
			r.Recorder.Eventf(node, corev1.EventTypeNormal, "RetainResourcePreserved",
				"Removed ownerReference from resource %s (policy=Retain)", resource.ID)
		}
	}

	return nil
}

// containsString checks if a string is in a slice
func containsString(slice []string, str string) bool {
	for _, item := range slice {
		if item == str {
			return true
		}
	}
	return false
}

// removeString removes a string from a slice
func removeString(slice []string, str string) []string {
	result := []string{}
	for _, item := range slice {
		if item != str {
			result = append(result, item)
		}
	}
	return result
}

// isNodeReady checks if a LynqNode has Ready condition set to True
func isNodeReady(node *lynqv1.LynqNode) bool {
	for _, cond := range node.Status.Conditions {
		if cond.Type == "Ready" {
			return cond.Status == metav1.ConditionTrue
		}
	}
	return false
}

// safetyMarginDuration is the minimum time we consider a node as "updating" after it starts updating.
// This covers the StatusManager batch delay (up to 1 second) plus a small safety buffer.
const safetyMarginDuration = 2 * time.Second

// countUpdatingNodes counts LynqNodes that are currently updating (updated to target generation but not Ready yet)
// This is used to enforce maxSkew - the maximum number of nodes that can be updating simultaneously
//
// A node is considered "updating" if ANY of the following conditions are true:
// 1. Template generation matches AND Ready condition is False
// 2. Template generation matches AND ObservedGeneration != Generation (controller hasn't reconciled yet)
// 3. Template generation matches AND recently updated (within safety margin) - handles async status updates
// 4. Template generation matches AND actual resources (Deployments, etc.) are not ready - direct cluster check
func (r *LynqHubReconciler) countUpdatingNodes(ctx context.Context, nodes []*lynqv1.LynqNode, targetGen int64) int32 {
	var count int32
	targetGenStr := fmt.Sprintf("%d", targetGen)
	logger := log.FromContext(ctx)

	for _, node := range nodes {
		nodeGen := node.Annotations[lynqv1.AnnotationTemplateGeneration]
		if nodeGen != targetGenStr {
			continue
		}

		// Case 1: Node is explicitly not Ready
		if !isNodeReady(node) {
			count++
			continue
		}

		// Case 2: LynqNode controller hasn't reconciled yet (status is stale)
		// This happens when LynqHub updates the spec but LynqNode controller hasn't processed it yet
		if node.Status.ObservedGeneration != node.Generation {
			count++
			continue
		}

		// Case 3: Recently updated - safety margin for async status updates
		// StatusManager batches updates with up to 1 second delay, so we add a 2 second margin
		if startTimeStr := node.Annotations[lynqv1.AnnotationRolloutUpdateStartTime]; startTimeStr != "" {
			if startTime, err := time.Parse(time.RFC3339, startTimeStr); err == nil {
				if time.Since(startTime) < safetyMarginDuration {
					count++
					continue
				}
			}
		}

		// Case 4: Actual resource readiness check (critical for slow-starting Pods)
		// Even if the LynqNode status says Ready and we're past the safety margin,
		// we still check the actual Deployment/StatefulSet/DaemonSet status in the cluster.
		// This ensures we don't start the next node's update before resources are truly ready.
		if !r.isNodeResourcesActuallyReady(ctx, node) {
			logger.V(1).Info("Node resources not actually ready",
				"lynqnode", node.Name, "namespace", node.Namespace)
			count++
			continue
		}
	}
	return count
}

// isNodeResourcesActuallyReady checks if all resources managed by a LynqNode are actually ready
// by querying the cluster directly instead of relying on cached LynqNode status.
// This is critical for maxSkew enforcement to ensure we don't start updating the next node
// before the current node's resources (especially Deployments with slow-starting Pods) are truly ready.
//
// This function only checks workload resources (Deployment, StatefulSet, DaemonSet) that have
// pods with potential startup delays. Other resources (ConfigMap, Secret, Service, etc.) are
// trusted to be ready based on the LynqNode status.
func (r *LynqHubReconciler) isNodeResourcesActuallyReady(ctx context.Context, node *lynqv1.LynqNode) bool {
	logger := log.FromContext(ctx)

	// If no applied resources tracked, trust the LynqNode status
	// This can happen for newly created nodes or nodes without workload resources
	if len(node.Status.AppliedResources) == 0 {
		return true
	}

	// Check each applied resource for workload types (Deployment, StatefulSet, DaemonSet)
	// Other resource types (ConfigMap, Secret, Service, etc.) are trusted as ready immediately
	for _, appliedResource := range node.Status.AppliedResources {
		kind, namespace, name := parseAppliedResource(appliedResource)
		if kind == "" || name == "" {
			continue
		}

		// Use the node's namespace if the resource namespace is empty
		if namespace == "" {
			namespace = node.Namespace
		}

		switch kind {
		case "Deployment":
			var deploy appsv1.Deployment
			if err := r.Get(ctx, types.NamespacedName{Namespace: namespace, Name: name}, &deploy); err != nil {
				if errors.IsNotFound(err) {
					logger.V(1).Info("Deployment not found, considering not ready",
						"deployment", name, "namespace", namespace, "lynqnode", node.Name)
				}
				return false
			}
			if !isDeploymentActuallyReady(&deploy) {
				logger.V(1).Info("Deployment not ready",
					"deployment", name, "namespace", namespace, "lynqnode", node.Name,
					"replicas", deploy.Spec.Replicas,
					"updatedReplicas", deploy.Status.UpdatedReplicas,
					"readyReplicas", deploy.Status.ReadyReplicas,
					"availableReplicas", deploy.Status.AvailableReplicas)
				return false
			}
		case "StatefulSet":
			var sts appsv1.StatefulSet
			if err := r.Get(ctx, types.NamespacedName{Namespace: namespace, Name: name}, &sts); err != nil {
				if errors.IsNotFound(err) {
					logger.V(1).Info("StatefulSet not found, considering not ready",
						"statefulset", name, "namespace", namespace, "lynqnode", node.Name)
				}
				return false
			}
			if !isStatefulSetActuallyReady(&sts) {
				logger.V(1).Info("StatefulSet not ready",
					"statefulset", name, "namespace", namespace, "lynqnode", node.Name,
					"replicas", sts.Spec.Replicas,
					"updatedReplicas", sts.Status.UpdatedReplicas,
					"readyReplicas", sts.Status.ReadyReplicas)
				return false
			}
		case "DaemonSet":
			var ds appsv1.DaemonSet
			if err := r.Get(ctx, types.NamespacedName{Namespace: namespace, Name: name}, &ds); err != nil {
				if errors.IsNotFound(err) {
					logger.V(1).Info("DaemonSet not found, considering not ready",
						"daemonset", name, "namespace", namespace, "lynqnode", node.Name)
				}
				return false
			}
			if !isDaemonSetActuallyReady(&ds) {
				logger.V(1).Info("DaemonSet not ready",
					"daemonset", name, "namespace", namespace, "lynqnode", node.Name,
					"desiredNumberScheduled", ds.Status.DesiredNumberScheduled,
					"updatedNumberScheduled", ds.Status.UpdatedNumberScheduled,
					"numberReady", ds.Status.NumberReady)
				return false
			}
		}
		// For other resource types (ConfigMap, Secret, Service, etc.), we trust the LynqNode status
		// as they are typically ready immediately after creation
	}

	return true
}

// parseAppliedResource parses the applied resource key format: "kind/namespace/name@id"
// Returns kind, namespace, name
func parseAppliedResource(key string) (kind, namespace, name string) {
	// Format: "kind/namespace/name@id"
	// Example: "Deployment/default/myapp@app-deployment"

	// Remove the @id suffix if present
	atIdx := strings.LastIndex(key, "@")
	if atIdx != -1 {
		key = key[:atIdx]
	}

	parts := strings.SplitN(key, "/", 3)
	if len(parts) < 2 {
		return "", "", ""
	}

	kind = parts[0]
	if len(parts) == 2 {
		// Format: "kind/name" (namespace is empty)
		name = parts[1]
		return kind, "", name
	}
	// Format: "kind/namespace/name"
	namespace = parts[1]
	name = parts[2]
	return kind, namespace, name
}

// isDeploymentActuallyReady checks if a Deployment is truly ready by examining its status directly.
// This is more stringent than checking the LynqNode status because:
// 1. LynqNode status may be cached or stale due to StatusManager batching
// 2. We need to ensure ALL replicas are updated and ready before proceeding
func isDeploymentActuallyReady(deploy *appsv1.Deployment) bool {
	// Check if controller has observed the latest spec
	if deploy.Generation != deploy.Status.ObservedGeneration {
		return false
	}

	replicas := int32(1)
	if deploy.Spec.Replicas != nil {
		replicas = *deploy.Spec.Replicas
	}

	// Deployment scaled to 0 is considered not ready for rolling update purposes
	if replicas == 0 {
		return false
	}

	// Basic replica count checks
	if deploy.Status.UpdatedReplicas != replicas ||
		deploy.Status.ReadyReplicas != replicas ||
		deploy.Status.AvailableReplicas != replicas {
		return false
	}

	// CRITICAL: Check the Progressing condition to detect rolling updates
	// During a rolling update with replicas=1:
	// - updatedReplicas=1 (new pod created)
	// - readyReplicas=1 (OLD pod still ready)
	// - availableReplicas=1 (OLD pod)
	// All counts match but rollout is NOT complete!
	//
	// The Progressing condition tells us the actual rollout status:
	// - Reason="NewReplicaSetAvailable" means rollout is COMPLETE
	// - Reason="ReplicaSetUpdated" means rollout is IN PROGRESS
	for _, cond := range deploy.Status.Conditions {
		if cond.Type == appsv1.DeploymentProgressing {
			// If Progressing is True with Reason "NewReplicaSetAvailable", rollout is complete
			// Any other reason means the rollout is still in progress
			if cond.Status == corev1.ConditionTrue && cond.Reason == "NewReplicaSetAvailable" {
				return true
			}
			// If Progressing condition exists but doesn't indicate completion, rollout is in progress
			return false
		}
	}

	// If no Progressing condition found, fall back to replica counts only
	// This shouldn't happen for a well-behaved Deployment controller
	return true
}

// isStatefulSetActuallyReady checks if a StatefulSet is truly ready
func isStatefulSetActuallyReady(sts *appsv1.StatefulSet) bool {
	// Check if controller has observed the latest spec
	if sts.Generation != sts.Status.ObservedGeneration {
		return false
	}

	replicas := int32(1)
	if sts.Spec.Replicas != nil {
		replicas = *sts.Spec.Replicas
	}

	// StatefulSet scaled to 0 is considered not ready
	if replicas == 0 {
		return false
	}

	// All conditions must be met
	return sts.Status.UpdatedReplicas == replicas &&
		sts.Status.ReadyReplicas == replicas &&
		sts.Status.CurrentReplicas == replicas
}

// isDaemonSetActuallyReady checks if a DaemonSet is truly ready
func isDaemonSetActuallyReady(ds *appsv1.DaemonSet) bool {
	// Check if controller has observed the latest spec
	if ds.Generation != ds.Status.ObservedGeneration {
		return false
	}

	// DaemonSet with no nodes to schedule is not ready
	if ds.Status.DesiredNumberScheduled == 0 {
		return false
	}

	// All conditions must be met
	return ds.Status.CurrentNumberScheduled == ds.Status.DesiredNumberScheduled &&
		ds.Status.UpdatedNumberScheduled == ds.Status.DesiredNumberScheduled &&
		ds.Status.NumberReady == ds.Status.DesiredNumberScheduled &&
		ds.Status.NumberAvailable == ds.Status.DesiredNumberScheduled
}

// canUpdateNode checks if we can update/create a node based on the template's maxSkew setting
// Returns true if maxSkew is not configured (unlimited) or if we're under the limit
func (r *LynqHubReconciler) canUpdateNode(ctx context.Context, tmpl *lynqv1.LynqForm, templateNodes []*lynqv1.LynqNode) bool {
	return r.canUpdateNodeWithCount(ctx, tmpl, templateNodes, 0)
}

// canUpdateNodeWithCount checks if we can update/create a node based on the template's maxSkew setting,
// taking into account additional updates made in the current reconcile iteration
// Returns true if maxSkew is not configured (unlimited) or if we're under the limit
func (r *LynqHubReconciler) canUpdateNodeWithCount(ctx context.Context, tmpl *lynqv1.LynqForm, templateNodes []*lynqv1.LynqNode, additionalUpdates int32) bool {
	// No rollout config means unlimited updates (current behavior)
	if tmpl.Spec.Rollout == nil || tmpl.Spec.Rollout.MaxSkew == 0 {
		return true
	}

	// Count nodes that are already updating based on their stored state
	updatingCount := r.countUpdatingNodes(ctx, templateNodes, tmpl.Generation)

	// Add the count of nodes we've updated in THIS iteration that aren't yet reflected in templateNodes
	totalUpdating := updatingCount + additionalUpdates

	return totalUpdating < tmpl.Spec.Rollout.MaxSkew
}

// SetupWithManager sets up the controller with the Manager.
func (r *LynqHubReconciler) SetupWithManager(mgr ctrl.Manager, concurrency int) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&lynqv1.LynqHub{}).
		Owns(&lynqv1.LynqNode{}).
		// Watch LynqForms to re-sync nodes when template changes
		Watches(&lynqv1.LynqForm{}, handler.EnqueueRequestsFromMapFunc(r.findRegistryForTemplate)).
		Named("lynqhub").
		WithOptions(controller.Options{
			MaxConcurrentReconciles: concurrency,
		}).
		Complete(r)
}

// findRegistryForTemplate maps a LynqForm to its LynqHub for watch events
func (r *LynqHubReconciler) findRegistryForTemplate(ctx context.Context, obj client.Object) []reconcile.Request {
	tmpl := obj.(*lynqv1.LynqForm)

	// Return a reconcile request for the hub referenced by this template
	return []reconcile.Request{
		{
			NamespacedName: types.NamespacedName{
				Name:      tmpl.Spec.HubID,
				Namespace: tmpl.Namespace,
			},
		},
	}
}
