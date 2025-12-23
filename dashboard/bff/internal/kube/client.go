package kube

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/clientcmd/api"
)

// Lynq CRD GVRs
var (
	LynqHubGVR = schema.GroupVersionResource{
		Group:    "operator.lynq.sh",
		Version:  "v1",
		Resource: "lynqhubs",
	}
	LynqFormGVR = schema.GroupVersionResource{
		Group:    "operator.lynq.sh",
		Version:  "v1",
		Resource: "lynqforms",
	}
	LynqNodeGVR = schema.GroupVersionResource{
		Group:    "operator.lynq.sh",
		Version:  "v1",
		Resource: "lynqnodes",
	}
	// Kubernetes core Events GVR
	EventsGVR = schema.GroupVersionResource{
		Group:    "",
		Version:  "v1",
		Resource: "events",
	}
)

// Client wraps the Kubernetes dynamic client
type Client struct {
	dynamic        dynamic.Interface
	discovery      discovery.DiscoveryInterface
	config         *rest.Config
	configLoader   clientcmd.ClientConfig
	kubeconfigPath string
	currentContext string
	appMode        string
	gvrCache       map[string]gvrCacheEntry
	gvrCacheMu     sync.RWMutex
}

// gvrCacheEntry holds cached GVR info
type gvrCacheEntry struct {
	GVR           schema.GroupVersionResource
	Namespaced    bool
}

// NewClient creates a new Kubernetes client
func NewClient(appMode, kubeconfigPath, contextName string) (*Client, error) {
	var config *rest.Config
	var configLoader clientcmd.ClientConfig
	var currentContext string
	var err error

	if appMode == "cluster" {
		// In-cluster config
		config, err = rest.InClusterConfig()
		if err != nil {
			return nil, fmt.Errorf("failed to get in-cluster config: %w", err)
		}
		currentContext = "in-cluster"
	} else {
		// Local mode - use kubeconfig
		// Priority: flag > KUBECONFIG env > default path
		if kubeconfigPath == "" {
			kubeconfigPath = os.Getenv("KUBECONFIG")
		}
		if kubeconfigPath == "" {
			kubeconfigPath = filepath.Join(os.Getenv("HOME"), ".kube", "config")
		}

		// Load raw config first
		loadingRules := &clientcmd.ClientConfigLoadingRules{ExplicitPath: kubeconfigPath}
		configOverrides := &clientcmd.ConfigOverrides{}
		tempLoader := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, configOverrides)

		rawConfig, err := tempLoader.RawConfig()
		if err != nil {
			return nil, fmt.Errorf("failed to load kubeconfig: %w", err)
		}

		// Determine which context to use
		if contextName != "" {
			// Validate that the specified context exists
			if _, exists := rawConfig.Contexts[contextName]; !exists {
				return nil, fmt.Errorf("context %q not found in kubeconfig", contextName)
			}
			currentContext = contextName
		} else if rawConfig.CurrentContext != "" {
			currentContext = rawConfig.CurrentContext
		}

		// If still no context, return client that can list contexts but not make API calls
		if currentContext == "" {
			return &Client{
				dynamic:        nil,
				config:         nil,
				configLoader:   tempLoader,
				kubeconfigPath: kubeconfigPath,
				currentContext: "",
				appMode:        appMode,
				gvrCache:       make(map[string]gvrCacheEntry),
			}, nil
		}

		// IMPORTANT: Modify raw config's current-context directly before creating client
		// This ensures the context is used even when kubeconfig has no current-context set
		rawConfig.CurrentContext = currentContext

		// Create client config from the modified raw config
		config, err = clientcmd.NewDefaultClientConfig(rawConfig, &clientcmd.ConfigOverrides{}).ClientConfig()
		if err != nil {
			return nil, fmt.Errorf("failed to get kubeconfig for context %s: %w", currentContext, err)
		}

		// Create config loader for future operations (like listing contexts)
		configOverrides.CurrentContext = currentContext
		configLoader = clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, configOverrides)
	}

	dynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create dynamic client: %w", err)
	}

	discoveryClient, err := discovery.NewDiscoveryClientForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create discovery client: %w", err)
	}

	return &Client{
		dynamic:        dynamicClient,
		discovery:      discoveryClient,
		config:         config,
		configLoader:   configLoader,
		kubeconfigPath: kubeconfigPath,
		currentContext: currentContext,
		appMode:        appMode,
		gvrCache:       make(map[string]gvrCacheEntry),
	}, nil
}

// ListContexts returns available kubeconfig contexts (local mode only)
func (c *Client) ListContexts() ([]KubeContext, error) {
	if c.appMode == "cluster" {
		return []KubeContext{{Name: "in-cluster", Current: true}}, nil
	}

	if c.configLoader == nil {
		return nil, fmt.Errorf("config loader not available")
	}

	rawConfig, err := c.configLoader.RawConfig()
	if err != nil {
		return nil, err
	}

	var contexts []KubeContext
	for name, ctx := range rawConfig.Contexts {
		contexts = append(contexts, KubeContext{
			Name:      name,
			Cluster:   ctx.Cluster,
			User:      ctx.AuthInfo,
			Namespace: ctx.Namespace,
			Current:   name == c.currentContext, // Use our tracked current context
		})
	}
	return contexts, nil
}

// SwitchContext changes the active kubeconfig context (local mode only)
func (c *Client) SwitchContext(contextName string) error {
	if c.appMode == "cluster" {
		return fmt.Errorf("context switching not supported in cluster mode")
	}

	loadingRules := &clientcmd.ClientConfigLoadingRules{ExplicitPath: c.kubeconfigPath}
	tempLoader := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, &clientcmd.ConfigOverrides{})

	rawConfig, err := tempLoader.RawConfig()
	if err != nil {
		return fmt.Errorf("failed to load kubeconfig: %w", err)
	}

	// Validate context exists
	if _, exists := rawConfig.Contexts[contextName]; !exists {
		return fmt.Errorf("context %q not found in kubeconfig", contextName)
	}

	// Set the context directly in raw config
	rawConfig.CurrentContext = contextName

	// Create client config from the modified raw config
	config, err := clientcmd.NewDefaultClientConfig(rawConfig, &clientcmd.ConfigOverrides{}).ClientConfig()
	if err != nil {
		return fmt.Errorf("failed to get kubeconfig for context %s: %w", contextName, err)
	}

	dynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		return err
	}

	discoveryClient, err := discovery.NewDiscoveryClientForConfig(config)
	if err != nil {
		return err
	}

	// Update config loader for future operations
	configOverrides := &clientcmd.ConfigOverrides{CurrentContext: contextName}
	c.configLoader = clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, configOverrides)
	c.dynamic = dynamicClient
	c.discovery = discoveryClient
	c.config = config
	c.currentContext = contextName
	// Clear GVR cache on context switch
	c.gvrCacheMu.Lock()
	c.gvrCache = make(map[string]gvrCacheEntry)
	c.gvrCacheMu.Unlock()
	return nil
}

// GetCurrentContext returns the current context name
func (c *Client) GetCurrentContext() string {
	return c.currentContext
}

// ListHubs lists all LynqHub resources
func (c *Client) ListHubs(ctx context.Context, namespace string) (*unstructured.UnstructuredList, error) {
	if c.dynamic == nil {
		return nil, fmt.Errorf("kubernetes client not initialized - no context selected")
	}
	if namespace == "" {
		return c.dynamic.Resource(LynqHubGVR).List(ctx, metav1.ListOptions{})
	}
	return c.dynamic.Resource(LynqHubGVR).Namespace(namespace).List(ctx, metav1.ListOptions{})
}

// GetHub gets a single LynqHub
func (c *Client) GetHub(ctx context.Context, name, namespace string) (*unstructured.Unstructured, error) {
	if c.dynamic == nil {
		return nil, fmt.Errorf("kubernetes client not initialized - no context selected")
	}
	if namespace == "" {
		return nil, fmt.Errorf("namespace is required")
	}
	return c.dynamic.Resource(LynqHubGVR).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
}

// FindHubByName finds a LynqHub by name across all namespaces
func (c *Client) FindHubByName(ctx context.Context, name string) (*unstructured.Unstructured, error) {
	if c.dynamic == nil {
		return nil, fmt.Errorf("kubernetes client not initialized - no context selected")
	}
	list, err := c.dynamic.Resource(LynqHubGVR).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	for _, item := range list.Items {
		if item.GetName() == name {
			return &item, nil
		}
	}
	return nil, fmt.Errorf("hub %q not found", name)
}

// ListForms lists all LynqForm resources
func (c *Client) ListForms(ctx context.Context, namespace string) (*unstructured.UnstructuredList, error) {
	if c.dynamic == nil {
		return nil, fmt.Errorf("kubernetes client not initialized - no context selected")
	}
	if namespace == "" {
		return c.dynamic.Resource(LynqFormGVR).List(ctx, metav1.ListOptions{})
	}
	return c.dynamic.Resource(LynqFormGVR).Namespace(namespace).List(ctx, metav1.ListOptions{})
}

// GetForm gets a single LynqForm
func (c *Client) GetForm(ctx context.Context, name, namespace string) (*unstructured.Unstructured, error) {
	if c.dynamic == nil {
		return nil, fmt.Errorf("kubernetes client not initialized - no context selected")
	}
	if namespace == "" {
		return nil, fmt.Errorf("namespace is required")
	}
	return c.dynamic.Resource(LynqFormGVR).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
}

// FindFormByName finds a LynqForm by name across all namespaces
func (c *Client) FindFormByName(ctx context.Context, name string) (*unstructured.Unstructured, error) {
	if c.dynamic == nil {
		return nil, fmt.Errorf("kubernetes client not initialized - no context selected")
	}
	list, err := c.dynamic.Resource(LynqFormGVR).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	for _, item := range list.Items {
		if item.GetName() == name {
			return &item, nil
		}
	}
	return nil, fmt.Errorf("form %q not found", name)
}

// ListNodes lists all LynqNode resources
func (c *Client) ListNodes(ctx context.Context, namespace string) (*unstructured.UnstructuredList, error) {
	if c.dynamic == nil {
		return nil, fmt.Errorf("kubernetes client not initialized - no context selected")
	}
	if namespace == "" {
		return c.dynamic.Resource(LynqNodeGVR).List(ctx, metav1.ListOptions{})
	}
	return c.dynamic.Resource(LynqNodeGVR).Namespace(namespace).List(ctx, metav1.ListOptions{})
}

// GetNode gets a single LynqNode
func (c *Client) GetNode(ctx context.Context, name, namespace string) (*unstructured.Unstructured, error) {
	if c.dynamic == nil {
		return nil, fmt.Errorf("kubernetes client not initialized - no context selected")
	}
	if namespace == "" {
		return nil, fmt.Errorf("namespace is required")
	}
	return c.dynamic.Resource(LynqNodeGVR).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
}

// FindNodeByName finds a LynqNode by name across all namespaces
func (c *Client) FindNodeByName(ctx context.Context, name string) (*unstructured.Unstructured, error) {
	if c.dynamic == nil {
		return nil, fmt.Errorf("kubernetes client not initialized - no context selected")
	}
	list, err := c.dynamic.Resource(LynqNodeGVR).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	for _, item := range list.Items {
		if item.GetName() == name {
			return &item, nil
		}
	}
	return nil, fmt.Errorf("node %q not found", name)
}

// ListNodesByHub lists nodes belonging to a specific hub
func (c *Client) ListNodesByHub(ctx context.Context, hubName, namespace string) (*unstructured.UnstructuredList, error) {
	if c.dynamic == nil {
		return nil, fmt.Errorf("kubernetes client not initialized - no context selected")
	}
	labelSelector := fmt.Sprintf("lynq.sh/hub=%s", hubName)
	if namespace == "" {
		return c.dynamic.Resource(LynqNodeGVR).List(ctx, metav1.ListOptions{LabelSelector: labelSelector})
	}
	return c.dynamic.Resource(LynqNodeGVR).Namespace(namespace).List(ctx, metav1.ListOptions{LabelSelector: labelSelector})
}

// KubeContext represents a kubeconfig context
type KubeContext struct {
	Name      string `json:"name"`
	Cluster   string `json:"cluster"`
	User      string `json:"user"`
	Namespace string `json:"namespace,omitempty"`
	Current   bool   `json:"current"`
}

// discoverGVR discovers the GVR for a given kind using the discovery API
func (c *Client) discoverGVR(kind string) (gvrCacheEntry, error) {
	// Check cache first
	c.gvrCacheMu.RLock()
	if entry, ok := c.gvrCache[kind]; ok {
		c.gvrCacheMu.RUnlock()
		return entry, nil
	}
	c.gvrCacheMu.RUnlock()

	if c.discovery == nil {
		return gvrCacheEntry{}, fmt.Errorf("discovery client not initialized")
	}

	// Get all API resources
	_, apiResourceLists, err := c.discovery.ServerGroupsAndResources()
	if err != nil {
		// Partial results are still useful
		if apiResourceLists == nil {
			return gvrCacheEntry{}, fmt.Errorf("failed to discover API resources: %w", err)
		}
	}

	// Search for the kind
	for _, apiResourceList := range apiResourceLists {
		if apiResourceList == nil {
			continue
		}
		gv, err := schema.ParseGroupVersion(apiResourceList.GroupVersion)
		if err != nil {
			continue
		}

		for _, apiResource := range apiResourceList.APIResources {
			// Skip subresources (e.g., pods/status)
			if strings.Contains(apiResource.Name, "/") {
				continue
			}

			if apiResource.Kind == kind {
				entry := gvrCacheEntry{
					GVR: schema.GroupVersionResource{
						Group:    gv.Group,
						Version:  gv.Version,
						Resource: apiResource.Name,
					},
					Namespaced: apiResource.Namespaced,
				}

				// Cache the result
				c.gvrCacheMu.Lock()
				c.gvrCache[kind] = entry
				c.gvrCacheMu.Unlock()

				return entry, nil
			}
		}
	}

	return gvrCacheEntry{}, fmt.Errorf("unknown resource kind: %s", kind)
}

// GetResource gets a single resource by kind, name, namespace
func (c *Client) GetResource(ctx context.Context, kind, name, namespace string) (*unstructured.Unstructured, error) {
	if c.dynamic == nil {
		return nil, fmt.Errorf("kubernetes client not initialized - no context selected")
	}

	entry, err := c.discoverGVR(kind)
	if err != nil {
		return nil, err
	}

	// Handle cluster-scoped resources
	if !entry.Namespaced {
		return c.dynamic.Resource(entry.GVR).Get(ctx, name, metav1.GetOptions{})
	}

	if namespace == "" {
		return nil, fmt.Errorf("namespace is required for %s", kind)
	}
	return c.dynamic.Resource(entry.GVR).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
}

// ListOrphanedResources lists resources with lynq.sh/orphaned=true label
func (c *Client) ListOrphanedResources(ctx context.Context, namespace string) (*unstructured.UnstructuredList, error) {
	if c.dynamic == nil {
		return nil, fmt.Errorf("kubernetes client not initialized - no context selected")
	}

	// Get common resource types that Lynq manages
	// We'll search these namespaced resources for orphaned label
	resourceTypes := []schema.GroupVersionResource{
		{Group: "apps", Version: "v1", Resource: "deployments"},
		{Group: "apps", Version: "v1", Resource: "statefulsets"},
		{Group: "apps", Version: "v1", Resource: "daemonsets"},
		{Group: "", Version: "v1", Resource: "services"},
		{Group: "", Version: "v1", Resource: "configmaps"},
		{Group: "", Version: "v1", Resource: "secrets"},
		{Group: "", Version: "v1", Resource: "serviceaccounts"},
		{Group: "", Version: "v1", Resource: "persistentvolumeclaims"},
		{Group: "batch", Version: "v1", Resource: "jobs"},
		{Group: "batch", Version: "v1", Resource: "cronjobs"},
		{Group: "networking.k8s.io", Version: "v1", Resource: "ingresses"},
	}

	labelSelector := "lynq.sh/orphaned=true"
	result := &unstructured.UnstructuredList{
		Items: []unstructured.Unstructured{},
	}

	for _, gvr := range resourceTypes {
		var list *unstructured.UnstructuredList
		var err error
		if namespace == "" {
			list, err = c.dynamic.Resource(gvr).List(ctx, metav1.ListOptions{LabelSelector: labelSelector})
		} else {
			list, err = c.dynamic.Resource(gvr).Namespace(namespace).List(ctx, metav1.ListOptions{LabelSelector: labelSelector})
		}
		if err != nil {
			// Skip resources that don't exist or can't be accessed
			continue
		}
		result.Items = append(result.Items, list.Items...)
	}

	return result, nil
}

// ListEvents lists Kubernetes events for a specific object
func (c *Client) ListEvents(ctx context.Context, namespace, involvedObjectName string) (*unstructured.UnstructuredList, error) {
	if c.dynamic == nil {
		return nil, fmt.Errorf("kubernetes client not initialized - no context selected")
	}

	opts := metav1.ListOptions{}
	if involvedObjectName != "" {
		opts.FieldSelector = fmt.Sprintf("involvedObject.name=%s", involvedObjectName)
	}

	if namespace == "" {
		return c.dynamic.Resource(EventsGVR).List(ctx, opts)
	}
	return c.dynamic.Resource(EventsGVR).Namespace(namespace).List(ctx, opts)
}

// Ensure api package is imported (used indirectly)
var _ = api.Config{}
