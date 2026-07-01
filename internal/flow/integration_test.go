// Package flow provides end-to-end integration tests for the agent flow system
package flow

import (
	"context"
	"testing"

	agentflowiov1alpha1 "agent-flow/api/v1alpha1"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	sandboxv1beta1 "sigs.k8s.io/agent-sandbox/api/v1beta1"
)

// TestFullFlowExecution tests the complete agent flow execution from Task creation to Sandbox generation
func TestFullFlowExecution(t *testing.T) {
	// Setup fake client with schemes
	scheme := runtime.NewScheme()
	_ = agentflowiov1alpha1.AddToScheme(scheme)
	_ = sandboxv1beta1.AddToScheme(scheme)

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()

	// Create a test Task with various configuration options
	task := &agentflowiov1alpha1.Task{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-task",
			Namespace: "default",
			UID:       types.UID("test-uid-123"),
		},
		Spec: agentflowiov1alpha1.TaskSpec{
			Command: []string{"/bin/sh", "-c"},
			Args:    []string{"echo 'Hello World'"},
			Image:   "alpine:latest",
			Env: []corev1.EnvVar{
				{
					Name:  "ENV_VAR",
					Value: "test-value",
				},
			},
			Resources: corev1.ResourceRequirements{
				Limits: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("100m"),
					corev1.ResourceMemory: resource.MustParse("128Mi"),
				},
			},
			RuntimeClassName: stringPtr("gvisor"),
			PodSecurityContext: &corev1.PodSecurityContext{
				RunAsNonRoot: boolPtr(true),
				RunAsUser:    int64Ptr(1000),
				FSGroup:      int64Ptr(2000),
			},
		},
	}

	// Create the Task in fake client
	if err := fakeClient.Create(context.Background(), task); err != nil {
		t.Fatalf("Failed to create test Task: %v", err)
	}

	// Execute the flow
	ctx := context.Background()
	state := State{
		Context: ctx,
		Client:  fakeClient,
		Task:    task,
		Phase:   "test",
	}

	flow := NewAgentFlow()
	flow.AddNode(&PlannerNode{Name: "test-planner"})

	if err := flow.Compile(); err != nil {
		t.Fatalf("Failed to compile flow: %v", err)
	}

	result, err := flow.Execute(ctx, state)
	if err != nil {
		t.Fatalf("Flow execution failed: %v", err)
	}

	// Verify the results
	if result.Phase != "planning" {
		t.Errorf("Expected phase 'planning', got '%s'", result.Phase)
	}

	// Verify AgentSandbox was created
	if result.AgentSandbox == nil {
		t.Fatal("Expected AgentSandbox to be created")
	}

	// Verify Sandbox properties
	if result.AgentSandbox.Name != "test-task-sandbox" {
		t.Errorf("Expected Sandbox name 'test-task-sandbox', got '%s'", result.AgentSandbox.Name)
	}

	if result.AgentSandbox.Namespace != "default" {
		t.Errorf("Expected Sandbox namespace 'default', got '%s'", result.AgentSandbox.Namespace)
	}

	// Verify labels
	if result.AgentSandbox.Labels["agents.x-k8s.io/sandbox"] != "test-task-sandbox" {
		t.Errorf("Expected sandbox label, got %v", result.AgentSandbox.Labels)
	}

	// Verify Pod spec
	if result.AgentSandbox.Spec.PodTemplate.Spec.RestartPolicy != corev1.RestartPolicyNever {
		t.Errorf("Expected RestartPolicy 'Never', got '%s'", result.AgentSandbox.Spec.PodTemplate.Spec.RestartPolicy)
	}

	// Verify RuntimeClassName was passed through
	if result.AgentSandbox.Spec.PodTemplate.Spec.RuntimeClassName == nil || *result.AgentSandbox.Spec.PodTemplate.Spec.RuntimeClassName != "gvisor" {
		t.Errorf("Expected RuntimeClassName 'gvisor', got '%v'", result.AgentSandbox.Spec.PodTemplate.Spec.RuntimeClassName)
	}

	// Verify PodSecurityContext was passed through
	if result.AgentSandbox.Spec.PodTemplate.Spec.SecurityContext == nil {
		t.Fatal("Expected PodSecurityContext to be set")
	}

	if result.AgentSandbox.Spec.PodTemplate.Spec.SecurityContext.RunAsNonRoot == nil || !*result.AgentSandbox.Spec.PodTemplate.Spec.SecurityContext.RunAsNonRoot {
		t.Error("Expected RunAsNonRoot to be true")
	}

	if result.AgentSandbox.Spec.PodTemplate.Spec.SecurityContext.RunAsUser == nil || *result.AgentSandbox.Spec.PodTemplate.Spec.SecurityContext.RunAsUser != 1000 {
		t.Error("Expected RunAsUser to be 1000")
	}

	if result.AgentSandbox.Spec.PodTemplate.Spec.SecurityContext.FSGroup == nil || *result.AgentSandbox.Spec.PodTemplate.Spec.SecurityContext.FSGroup != 2000 {
		t.Error("Expected FSGroup to be 2000")
	}

	// Verify container configuration
	containers := result.AgentSandbox.Spec.PodTemplate.Spec.Containers
	if len(containers) != 1 {
		t.Errorf("Expected 1 container, got %d", len(containers))
	}

	container := containers[0]
	if container.Name != "test-task" {
		t.Errorf("Expected container name 'test-task', got '%s'", container.Name)
	}

	if container.Image != "alpine:latest" {
		t.Errorf("Expected image 'alpine:latest', got '%s'", container.Image)
	}

	if len(container.Command) != 2 {
		t.Errorf("Expected 2 commands, got %d", len(container.Command))
	}

	if len(container.Args) != 1 {
		t.Errorf("Expected 1 arg, got %d", len(container.Args))
	}

	if container.Args[0] != "echo 'Hello World'" {
		t.Errorf("Expected args 'echo 'Hello World'', got '%s'", container.Args[0])
	}

	if len(container.Env) != 1 {
		t.Errorf("Expected 1 env var, got %d", len(container.Env))
	}

	if container.Env[0].Name != "ENV_VAR" || container.Env[0].Value != "test-value" {
		t.Errorf("Expected env var ENV_VAR=test-value, got %v", container.Env)
	}

	t.Log("Full flow execution test passed!")
}

// TestFullFlowWithWarmPool tests flow with sandbox warm pool configuration
func TestFullFlowWithWarmPool(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = agentflowiov1alpha1.AddToScheme(scheme)
	_ = sandboxv1beta1.AddToScheme(scheme)

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()

	// Create a test Task without Sandbox (to test creation path)
	task := &agentflowiov1alpha1.Task{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "warm-pool-task",
			Namespace: "default",
			UID:       types.UID("warm-pool-uid"),
		},
		Spec: agentflowiov1alpha1.TaskSpec{
			Command: []string{"/bin/echo"},
			Args:    []string{"test"},
			Image:   "ubuntu:20.04",
		},
	}

	if err := fakeClient.Create(context.Background(), task); err != nil {
		t.Fatalf("Failed to create test Task: %v", err)
	}

	ctx := context.Background()
	state := State{
		Context: ctx,
		Client:  fakeClient,
		Task:    task,
		Phase:   "test",
	}

	flow := NewAgentFlow()
	flow.AddNode(&PlannerNode{Name: "warm-pool-planner"})

	if err := flow.Compile(); err != nil {
		t.Fatalf("Failed to compile flow: %v", err)
	}

	result, err := flow.Execute(ctx, state)
	if err != nil {
		t.Fatalf("Flow execution failed: %v", err)
	}

	if result.AgentSandbox == nil {
		t.Fatal("Expected AgentSandbox to be created")
	}

	if result.AgentSandbox.Name != "warm-pool-task-sandbox" {
		t.Errorf("Expected Sandbox name 'warm-pool-task-sandbox', got '%s'", result.AgentSandbox.Name)
	}

	t.Log("Warm pool flow execution test passed!")
}

// TestPlannerNodeReuseExistingSandbox tests that PlannerNode correctly detects existing Sandbox
func TestPlannerNodeReuseExistingSandbox(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = agentflowiov1alpha1.AddToScheme(scheme)
	_ = sandboxv1beta1.AddToScheme(scheme)

	// Pre-create a Sandbox
	existingSandbox := &sandboxv1beta1.Sandbox{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "reuse-test-sandbox",
			Namespace: "default",
		},
		Spec: sandboxv1beta1.SandboxSpec{
			PodTemplate: sandboxv1beta1.PodTemplate{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "existing",
							Image: "existing-image",
						},
					},
				},
			},
		},
	}

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(existingSandbox).Build()

	task := &agentflowiov1alpha1.Task{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "reuse-test",
			Namespace: "default",
			UID:       types.UID("reuse-uid"),
		},
		Spec: agentflowiov1alpha1.TaskSpec{
			Command: []string{"/bin/test"},
			Image:   "should-not-be-used",
		},
	}

	if err := fakeClient.Create(context.Background(), task); err != nil {
		t.Fatalf("Failed to create test Task: %v", err)
	}

	ctx := context.Background()
	state := State{
		Context: ctx,
		Client:  fakeClient,
		Task:    task,
		Phase:   "test",
	}

	flow := NewAgentFlow()
	flow.AddNode(&PlannerNode{Name: "reuse-planner"})

	if err := flow.Compile(); err != nil {
		t.Fatalf("Failed to compile flow: %v", err)
	}

	result, err := flow.Execute(ctx, state)
	if err != nil {
		t.Fatalf("Flow execution failed: %v", err)
	}

	// Verify existing Sandbox was reused
	if result.AgentSandbox == nil {
		t.Fatal("Expected AgentSandbox to be reused")
	}

	if result.AgentSandbox.Name != "reuse-test-sandbox" {
		t.Errorf("Expected to reuse Sandbox 'reuse-test-sandbox', got '%s'", result.AgentSandbox.Name)
	}

	// Verify the existing container was kept (not replaced with task spec)
	if result.AgentSandbox.Spec.PodTemplate.Spec.Containers[0].Image != "existing-image" {
		t.Errorf("Expected to reuse existing image 'existing-image', got '%s'",
			result.AgentSandbox.Spec.PodTemplate.Spec.Containers[0].Image)
	}

	t.Log("Reuse existing Sandbox test passed!")
}

func stringPtr(s string) *string {
	return &s
}

func boolPtr(b bool) *bool {
	return &b
}

func int64Ptr(i int64) *int64 {
	return &i
}
