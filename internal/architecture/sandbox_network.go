package architecture

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"

	agentflowiov1alpha1 "agent-flow/api/v1alpha1"

	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	networkPolicyLabel      = "agentflow.io/network-policy"
	networkPolicyManaged    = "agentflow.io/managed-by"
	networkPolicyManagedVal = "agentflow-controller"

	NetworkPolicyOff        = "off"
	NetworkPolicyStrict     = "strict"
	NetworkPolicyPermissive = "permissive"
)

// SandboxNetworkConfig controls sandbox egress policy generation.
type SandboxNetworkConfig struct {
	Enabled         bool
	DefaultMode     string
	DNSNamespace    string
	AllowPublicHTTP bool
	ExtraCIDRs      []string
	AIEndpointCIDRs []string
	AIPorts         []int32
}

// LoadSandboxNetworkConfig reads network policy settings from environment.
func LoadSandboxNetworkConfig() SandboxNetworkConfig {
	cfg := SandboxNetworkConfig{
		Enabled:         envBoolDefault("SANDBOX_NETWORK_POLICY_ENABLED", true),
		DefaultMode:     envStringDefault("SANDBOX_NETWORK_POLICY_MODE", NetworkPolicyStrict),
		DNSNamespace:    envStringDefault("SANDBOX_DNS_NAMESPACE", "kube-system"),
		AllowPublicHTTP: envBoolDefault("SANDBOX_ALLOW_PUBLIC_EGRESS", false),
		ExtraCIDRs:      splitCSV(os.Getenv("SANDBOX_EGRESS_EXTRA_CIDRS")),
		AIEndpointCIDRs: splitCSV(os.Getenv("SANDBOX_AI_EGRESS_CIDRS")),
		AIPorts:         parsePortsEnv(os.Getenv("SANDBOX_AI_EGRESS_PORTS")),
	}

	if aiBase := os.Getenv("AI_BASE_URL"); aiBase != "" {
		cfg.mergeAIEndpoint(aiBase)
	}
	return cfg
}

func (c *SandboxNetworkConfig) mergeAIEndpoint(raw string) {
	host, port, err := parseURLHostPort(raw)
	if err != nil {
		return
	}
	if ip := net.ParseIP(host); ip != nil {
		bits := "32"
		if ip.To4() == nil {
			bits = "128"
		}
		cidr := fmt.Sprintf("%s/%s", ip.String(), bits)
		c.AIEndpointCIDRs = appendUnique(c.AIEndpointCIDRs, cidr)
	}
	if port > 0 {
		c.AIPorts = appendUniqueInt32(c.AIPorts, port)
	}
}

func (p *TaskPlannerEino) sandboxNetworkConfig() SandboxNetworkConfig {
	if p.networkConfig != nil {
		return *p.networkConfig
	}
	cfg := LoadSandboxNetworkConfig()
	p.networkConfig = &cfg
	return cfg
}

func sandboxNetworkPolicyName(taskName string) string {
	return fmt.Sprintf("sandbox-net-%s", taskName)
}

func resolveNetworkPolicyMode(task *agentflowiov1alpha1.Task, cfg SandboxNetworkConfig) string {
	if task.Annotations != nil {
		if mode := strings.ToLower(strings.TrimSpace(task.Annotations[networkPolicyLabel])); mode != "" {
			return mode
		}
	}
	return strings.ToLower(cfg.DefaultMode)
}

func (p *TaskPlannerEino) ensureSandboxNetworkPolicy(ctx context.Context, task *agentflowiov1alpha1.Task) error {
	cfg := p.sandboxNetworkConfig()
	mode := resolveNetworkPolicyMode(task, cfg)
	if !cfg.Enabled || mode == NetworkPolicyOff {
		return nil
	}

	np := buildSandboxNetworkPolicy(task, cfg, mode)
	existing := &networkingv1.NetworkPolicy{}
	key := types.NamespacedName{Name: np.Name, Namespace: np.Namespace}
	err := p.Get(ctx, key, existing)
	if k8serrors.IsNotFound(err) {
		return p.Create(ctx, np)
	}
	if err != nil {
		return err
	}

	existing.Spec = np.Spec
	existing.Labels = np.Labels
	existing.Annotations = np.Annotations
	return p.Update(ctx, existing)
}

func (p *TaskPlannerEino) deleteSandboxNetworkPolicy(ctx context.Context, task *agentflowiov1alpha1.Task) {
	np := &networkingv1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      sandboxNetworkPolicyName(task.Name),
			Namespace: task.Namespace,
		},
	}
	if err := p.Delete(ctx, np); err != nil && !k8serrors.IsNotFound(err) {
		log.FromContext(ctx).WithName("network-policy").Error(err, "failed to delete network policy",
			"namespace", task.Namespace, "name", np.Name)
	}
}

func buildSandboxNetworkPolicy(task *agentflowiov1alpha1.Task, cfg SandboxNetworkConfig, mode string) *networkingv1.NetworkPolicy {
	permissive := mode == NetworkPolicyPermissive || cfg.AllowPublicHTTP

	egress := []networkingv1.NetworkPolicyEgressRule{
		dnsEgressRule(cfg.DNSNamespace),
	}

	extraCIDRs := append([]string{}, cfg.ExtraCIDRs...)
	extraCIDRs = appendUnique(extraCIDRs, cfg.AIEndpointCIDRs...)
	if task.Annotations != nil {
		extraCIDRs = appendUnique(extraCIDRs, splitCSV(task.Annotations["agentflow.io/egress-cidrs"])...)
	}

	ports := append([]int32{}, cfg.AIPorts...)
	if task.Annotations != nil {
		ports = appendUniqueInt32(ports, parsePortsEnv(task.Annotations["agentflow.io/egress-ports"])...)
	}
	if len(ports) == 0 {
		ports = []int32{443, 80, 9101}
	}

	if len(extraCIDRs) > 0 {
		egress = append(egress, cidrEgressRules(extraCIDRs, ports)...)
	}

	if permissive {
		egress = append(egress, publicEgressRules([]int32{443, 80})...)
	}

	return &networkingv1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      sandboxNetworkPolicyName(task.Name),
			Namespace: task.Namespace,
			Labels: map[string]string{
				networkPolicyManaged:    networkPolicyManagedVal,
				"agentflow.io/task":     task.Name,
				"agentflow.io/sandbox":  "true",
				"agentflow.io/net-mode": mode,
			},
			Annotations: map[string]string{
				"agentflow.io/task-uid": string(task.UID),
			},
		},
		Spec: networkingv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{
				MatchLabels: map[string]string{
					"agentflow.io/task": task.Name,
				},
			},
			PolicyTypes: []networkingv1.PolicyType{
				networkingv1.PolicyTypeIngress,
				networkingv1.PolicyTypeEgress,
			},
			Ingress: []networkingv1.NetworkPolicyIngressRule{},
			Egress:  egress,
		},
	}
}

func dnsEgressRule(namespace string) networkingv1.NetworkPolicyEgressRule {
	if namespace == "" {
		namespace = "kube-system"
	}
	return networkingv1.NetworkPolicyEgressRule{
		To: []networkingv1.NetworkPolicyPeer{
			{
				NamespaceSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"kubernetes.io/metadata.name": namespace,
					},
				},
			},
		},
		Ports: []networkingv1.NetworkPolicyPort{
			{Protocol: protocolPtr(corev1.ProtocolUDP), Port: intstrPtr(53)},
			{Protocol: protocolPtr(corev1.ProtocolTCP), Port: intstrPtr(53)},
		},
	}
}

func cidrEgressRules(cidrs []string, ports []int32) []networkingv1.NetworkPolicyEgressRule {
	var rules []networkingv1.NetworkPolicyEgressRule
	for _, cidr := range cidrs {
		cidr = strings.TrimSpace(cidr)
		if cidr == "" {
			continue
		}
		if _, _, err := net.ParseCIDR(cidr); err != nil {
			if ip := net.ParseIP(cidr); ip != nil {
				bits := "32"
				if ip.To4() == nil {
					bits = "128"
				}
				cidr = fmt.Sprintf("%s/%s", ip.String(), bits)
			} else {
				continue
			}
		}
		rule := networkingv1.NetworkPolicyEgressRule{
			To: []networkingv1.NetworkPolicyPeer{
				{IPBlock: &networkingv1.IPBlock{CIDR: cidr}},
			},
		}
		for _, port := range ports {
			rule.Ports = append(rule.Ports, networkingv1.NetworkPolicyPort{
				Protocol: protocolPtr(corev1.ProtocolTCP),
				Port:     intstrPtr(port),
			})
		}
		rules = append(rules, rule)
	}
	return rules
}

func publicEgressRules(ports []int32) []networkingv1.NetworkPolicyEgressRule {
	var rules []networkingv1.NetworkPolicyEgressRule
	for _, cidr := range []string{"0.0.0.0/0", "::/0"} {
		rule := networkingv1.NetworkPolicyEgressRule{
			To: []networkingv1.NetworkPolicyPeer{
				{IPBlock: &networkingv1.IPBlock{CIDR: cidr}},
			},
		}
		for _, port := range ports {
			rule.Ports = append(rule.Ports, networkingv1.NetworkPolicyPort{
				Protocol: protocolPtr(corev1.ProtocolTCP),
				Port:     intstrPtr(port),
			})
		}
		rules = append(rules, rule)
	}
	return rules
}

func sandboxPodLabels(task *agentflowiov1alpha1.Task, sandboxName string) map[string]string {
	return map[string]string{
		"agents.x-k8s.io/sandbox": sandboxName,
		"agentflow.io/task":       task.Name,
		"agentflow.io/sandbox":    "true",
	}
}

func applySandboxPodDefaults(spec *corev1.PodSpec) {
	if spec == nil {
		return
	}
	if spec.DNSPolicy == "" {
		spec.DNSPolicy = corev1.DNSClusterFirst
	}
}

func parseURLHostPort(raw string) (string, int32, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return "", 0, err
	}
	host := u.Hostname()
	if host == "" {
		return "", 0, fmt.Errorf("missing host in url")
	}
	port := int32(0)
	if p := u.Port(); p != "" {
		v, err := strconv.Atoi(p)
		if err != nil {
			return "", 0, err
		}
		port = int32(v)
	} else if u.Scheme == "https" {
		port = 443
	} else {
		port = 80
	}
	return host, port, nil
}

func splitCSV(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func parsePortsEnv(raw string) []int32 {
	var ports []int32
	for _, part := range splitCSV(raw) {
		v, err := strconv.Atoi(part)
		if err != nil || v <= 0 || v > 65535 {
			continue
		}
		ports = append(ports, int32(v))
	}
	return ports
}

func appendUnique(items []string, values ...string) []string {
	seen := make(map[string]bool, len(items))
	for _, item := range items {
		seen[item] = true
	}
	for _, value := range values {
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		items = append(items, value)
	}
	return items
}

func appendUniqueInt32(items []int32, values ...int32) []int32 {
	seen := make(map[int32]bool, len(items))
	for _, item := range items {
		seen[item] = true
	}
	for _, value := range values {
		if value <= 0 || seen[value] {
			continue
		}
		seen[value] = true
		items = append(items, value)
	}
	return items
}

func envStringDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envBoolDefault(key string, fallback bool) bool {
	raw := strings.ToLower(strings.TrimSpace(os.Getenv(key)))
	if raw == "" {
		return fallback
	}
	return raw == "1" || raw == "true" || raw == "yes" || raw == "on"
}

func protocolPtr(p corev1.Protocol) *corev1.Protocol { return &p }
func intstrPtr(port int32) *intstr.IntOrString {
	v := intstr.FromInt32(port)
	return &v
}
