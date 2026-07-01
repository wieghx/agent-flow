package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
)

// Common resource mappings for convenience
var resourceAliases = map[string]schema.GroupVersionResource{
	"pods":                   {Group: "", Version: "v1", Resource: "pods"},
	"pod":                    {Group: "", Version: "v1", Resource: "pods"},
	"services":               {Group: "", Version: "v1", Resource: "services"},
	"service":                {Group: "", Version: "v1", Resource: "services"},
	"deployments":            {Group: "apps", Version: "v1", Resource: "deployments"},
	"deployment":             {Group: "apps", Version: "v1", Resource: "deployments"},
	"replicasets":            {Group: "apps", Version: "v1", Resource: "replicasets"},
	"daemonsets":             {Group: "apps", Version: "v1", Resource: "daemonsets"},
	"statefulsets":           {Group: "apps", Version: "v1", Resource: "statefulsets"},
	"jobs":                   {Group: "batch", Version: "v1", Resource: "jobs"},
	"cronjobs":               {Group: "batch", Version: "v1", Resource: "cronjobs"},
	"configmaps":             {Group: "", Version: "v1", Resource: "configmaps"},
	"configmap":              {Group: "", Version: "v1", Resource: "configmaps"},
	"secrets":                {Group: "", Version: "v1", Resource: "secrets"},
	"secret":                 {Group: "", Version: "v1", Resource: "secrets"},
	"namespaces":             {Group: "", Version: "v1", Resource: "namespaces"},
	"namespace":              {Group: "", Version: "v1", Resource: "namespaces"},
	"nodes":                  {Group: "", Version: "v1", Resource: "nodes"},
	"node":                   {Group: "", Version: "v1", Resource: "nodes"},
	"events":                 {Group: "", Version: "v1", Resource: "events"},
	"persistentvolumeclaims": {Group: "", Version: "v1", Resource: "persistentvolumeclaims"},
	"pvc":                    {Group: "", Version: "v1", Resource: "persistentvolumeclaims"},
	"persistentvolumes":      {Group: "", Version: "v1", Resource: "persistentvolumes"},
	"pv":                     {Group: "", Version: "v1", Resource: "persistentvolumes"},
	"serviceaccounts":        {Group: "", Version: "v1", Resource: "serviceaccounts"},
	"serviceaccount":         {Group: "", Version: "v1", Resource: "serviceaccounts"},
	"tasks":                  {Group: "agentflow.io", Version: "v1alpha1", Resource: "tasks"},
	"task":                   {Group: "agentflow.io", Version: "v1alpha1", Resource: "tasks"},
	"sandboxes":              {Group: "agents.x-k8s.io", Version: "v1beta1", Resource: "sandboxes"},
	"sandbox":                {Group: "agents.x-k8s.io", Version: "v1beta1", Resource: "sandboxes"},
	"ingresses":              {Group: "networking.k8s.io", Version: "v1", Resource: "ingresses"},
	"ingress":                {Group: "networking.k8s.io", Version: "v1", Resource: "ingresses"},
	"networkpolicies":        {Group: "networking.k8s.io", Version: "v1", Resource: "networkpolicies"},
	"roles":                  {Group: "rbac.authorization.k8s.io", Version: "v1", Resource: "roles"},
	"rolebindings":           {Group: "rbac.authorization.k8s.io", Version: "v1", Resource: "rolebindings"},
	"clusterroles":           {Group: "rbac.authorization.k8s.io", Version: "v1", Resource: "clusterroles"},
	"clusterrolebindings":    {Group: "rbac.authorization.k8s.io", Version: "v1", Resource: "clusterrolebindings"},
	"pods/log":               {Group: "", Version: "v1", Resource: "pods/log"},
}

type K8sGetTool struct {
	client dynamic.Interface
}

func NewK8sGetTool() (*K8sGetTool, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to get in-cluster config: %w (k8s_get requires running inside a K8s Pod)", err)
	}
	dynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create dynamic client: %w", err)
	}
	return &K8sGetTool{client: dynamicClient}, nil
}

func (t *K8sGetTool) Name() string { return "k8s_get" }
func (t *K8sGetTool) Description() string {
	return `Query Kubernetes resources. Input: {"resource": "pods", "namespace": "default", "name": "my-pod", "labelSelector": "app=nginx"}
- resource: resource type (pods, services, deployments, configmaps, secrets, nodes, tasks, sandboxes, etc.)
- namespace: namespace (default="default", use "" for cluster-scoped resources like nodes)
- name: specific resource name (optional, omit to list)
- labelSelector: label filter (optional, e.g. "app=nginx")
- limit: max items to list (default 50)`
}

func (t *K8sGetTool) Execute(ctx context.Context, input map[string]interface{}) (string, error) {
	resourceStr, _ := input["resource"].(string)
	namespace, _ := input["namespace"].(string)
	name, _ := input["name"].(string)
	labelSelector, _ := input["labelSelector"].(string)
	limit := 50
	if l, ok := input["limit"].(float64); ok && l > 0 {
		limit = int(l)
	}

	if resourceStr == "" {
		return "", fmt.Errorf("resource is required")
	}

	gvr, ok := resourceAliases[strings.ToLower(resourceStr)]
	if !ok {
		return "", fmt.Errorf("unknown resource type: %s. Supported: pods, services, deployments, configmaps, secrets, nodes, tasks, sandboxes, etc.", resourceStr)
	}

	// Cluster-scoped resources don't use namespace
	switch gvr.Resource {
	case "nodes", "namespaces", "persistentvolumes", "clusterroles", "clusterrolebindings":
		namespace = ""
	}

	if namespace == "" {
		namespace = "default"
	}

	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	var result interface{}

	if name != "" {
		// Get single resource
		var dynClient dynamic.ResourceInterface
		if namespace == "" {
			dynClient = t.client.Resource(gvr)
		} else {
			dynClient = t.client.Resource(gvr).Namespace(namespace)
		}

		obj, err := dynClient.Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return "", fmt.Errorf("get %s/%s failed: %w", gvr.Resource, name, err)
		}
		result = formatUnstructured(obj)
	} else {
		// List resources
		var dynClient dynamic.ResourceInterface
		if namespace == "" {
			dynClient = t.client.Resource(gvr)
		} else {
			dynClient = t.client.Resource(gvr).Namespace(namespace)
		}

		listOpts := metav1.ListOptions{
			Limit: int64(limit),
		}
		if labelSelector != "" {
			listOpts.LabelSelector = labelSelector
		}

		list, err := dynClient.List(ctx, listOpts)
		if err != nil {
			return "", fmt.Errorf("list %s failed: %w", gvr.Resource, err)
		}

		items := make([]interface{}, 0, len(list.Items))
		for _, item := range list.Items {
			items = append(items, formatUnstructured(&item))
		}
		result = map[string]interface{}{
			"total":     len(list.Items),
			"truncated": len(list.Items) >= limit,
			"items":     items,
		}
	}

	output, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal result failed: %w", err)
	}
	return string(output), nil
}

func formatUnstructured(obj *unstructured.Unstructured) map[string]interface{} {
	metadata := obj.Object["metadata"].(map[string]interface{})
	status, _ := obj.Object["status"].(map[string]interface{})
	spec, _ := obj.Object["spec"].(map[string]interface{})

	result := map[string]interface{}{
		"name":              metadata["name"],
		"namespace":         metadata["namespace"],
		"labels":            metadata["labels"],
		"creationTimestamp": metadata["creationTimestamp"],
	}

	if uid, ok := metadata["uid"].(string); ok {
		result["uid"] = uid
	}
	if rv, ok := metadata["resourceVersion"].(string); ok {
		result["resourceVersion"] = rv
	}

	if spec != nil {
		result["spec"] = spec
	}
	if status != nil {
		result["status"] = status
	}

	return result
}

// Ensure K8sGetTool implements Tool
var _ Tool = (*K8sGetTool)(nil)
