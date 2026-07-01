package architecture

import (
	"testing"

	agentflowiov1alpha1 "agent-flow/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestBuildSandboxNetworkPolicyStrict(t *testing.T) {
	task := &agentflowiov1alpha1.Task{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "demo-task",
			Namespace: "default",
			UID:       "uid-1",
		},
	}
	cfg := SandboxNetworkConfig{
		Enabled:         true,
		DefaultMode:     NetworkPolicyStrict,
		DNSNamespace:    "kube-system",
		AIEndpointCIDRs: []string{"203.0.113.10/32"},
		AIPorts:         []int32{9101},
	}

	np := buildSandboxNetworkPolicy(task, cfg, NetworkPolicyStrict)
	if np.Name != "sandbox-net-demo-task" {
		t.Fatalf("unexpected name: %s", np.Name)
	}
	if np.Spec.PodSelector.MatchLabels["agentflow.io/task"] != "demo-task" {
		t.Fatalf("unexpected selector: %+v", np.Spec.PodSelector)
	}
	if len(np.Spec.Egress) < 2 {
		t.Fatalf("expected dns + ai egress rules, got %d", len(np.Spec.Egress))
	}
}

func TestBuildSandboxNetworkPolicyPermissive(t *testing.T) {
	task := &agentflowiov1alpha1.Task{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "demo-task",
			Namespace: "default",
		},
	}
	cfg := SandboxNetworkConfig{
		DNSNamespace: "kube-system",
	}
	np := buildSandboxNetworkPolicy(task, cfg, NetworkPolicyPermissive)
	foundPublic := false
	for _, rule := range np.Spec.Egress {
		for _, peer := range rule.To {
			if peer.IPBlock != nil && peer.IPBlock.CIDR == "0.0.0.0/0" {
				foundPublic = true
			}
		}
	}
	if !foundPublic {
		t.Fatal("permissive policy should allow public egress")
	}
}

func TestResolveNetworkPolicyModeFromAnnotation(t *testing.T) {
	task := &agentflowiov1alpha1.Task{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{
				networkPolicyLabel: NetworkPolicyOff,
			},
		},
	}
	mode := resolveNetworkPolicyMode(task, SandboxNetworkConfig{DefaultMode: NetworkPolicyStrict})
	if mode != NetworkPolicyOff {
		t.Fatalf("mode = %q, want off", mode)
	}
}

func TestLoadSandboxNetworkConfigParsesAIURL(t *testing.T) {
	t.Setenv("SANDBOX_NETWORK_POLICY_ENABLED", "true")
	t.Setenv("AI_BASE_URL", "http://203.0.113.20:9101")
	cfg := LoadSandboxNetworkConfig()
	if len(cfg.AIEndpointCIDRs) != 1 || cfg.AIEndpointCIDRs[0] != "203.0.113.20/32" {
		t.Fatalf("AI cidr not parsed: %+v", cfg.AIEndpointCIDRs)
	}
	if len(cfg.AIPorts) != 1 || cfg.AIPorts[0] != 9101 {
		t.Fatalf("AI port not parsed: %+v", cfg.AIPorts)
	}
}
