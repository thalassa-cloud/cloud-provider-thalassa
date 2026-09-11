package provider

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/thalassa-cloud/client-go/iaas"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

func TestShouldAllowICMP(t *testing.T) {
	lb := &loadbalancer{}

	tests := []struct {
		name     string
		service  *corev1.Service
		expected bool
	}{
		{
			name: "annotation true",
			service: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						LoadBalancerAnnotationSecurityGroupAllowICMP: "true",
					},
				},
			},
			expected: true,
		},
		{
			name: "annotation false",
			service: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						LoadBalancerAnnotationSecurityGroupAllowICMP: "false",
					},
				},
			},
			expected: false,
		},
		{
			name: "annotation missing defaults to false",
			service: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{},
				},
			},
			expected: false,
		},
		{
			name: "invalid annotation defaults to false",
			service: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						LoadBalancerAnnotationSecurityGroupAllowICMP: "not-a-bool",
					},
				},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, lb.shouldAllowICMP(tt.service))
		})
	}
}

func TestGetIcmpAllowedSources(t *testing.T) {
	lb := &loadbalancer{}

	tests := []struct {
		name            string
		service         *corev1.Service
		expectedSources []string
	}{
		{
			name: "custom icmp sources take precedence over ACL",
			service: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						LoadBalancerAnnotationSecurityGroupICMPAllowedSources: "203.0.113.0/24",
						LoadbalancerAnnotationAclAllowedSources:               "10.0.0.0/8",
					},
				},
				Spec: corev1.ServiceSpec{
					LoadBalancerSourceRanges: []string{"198.51.100.0/24"},
				},
			},
			expectedSources: []string{"203.0.113.0/24"},
		},
		{
			name: "inherits from acl-allowed-sources when icmp sources unset",
			service: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						LoadbalancerAnnotationAclAllowedSources: "10.0.0.0/8,192.168.1.0/24",
					},
				},
			},
			expectedSources: []string{"10.0.0.0/8", "192.168.1.0/24"},
		},
		{
			name: "inherits from loadBalancerSourceRanges when icmp sources unset",
			service: &corev1.Service{
				Spec: corev1.ServiceSpec{
					LoadBalancerSourceRanges: []string{"203.0.113.0/24", "198.51.100.0/24"},
				},
			},
			expectedSources: []string{"203.0.113.0/24", "198.51.100.0/24"},
		},
		{
			name: "inherits combined global ACL sources",
			service: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						LoadbalancerAnnotationAclAllowedSources: "10.0.0.0/8",
					},
				},
				Spec: corev1.ServiceSpec{
					LoadBalancerSourceRanges: []string{"203.0.113.0/24", "10.0.0.0/8"},
				},
			},
			expectedSources: []string{"203.0.113.0/24", "10.0.0.0/8"},
		},
		{
			name: "explicit empty icmp sources does not inherit",
			service: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						LoadBalancerAnnotationSecurityGroupICMPAllowedSources: "",
						LoadbalancerAnnotationAclAllowedSources:               "10.0.0.0/8",
					},
				},
			},
			expectedSources: []string{},
		},
		{
			name: "invalid icmp CIDRs are skipped",
			service: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						LoadBalancerAnnotationSecurityGroupICMPAllowedSources: "203.0.113.0/24,not-a-cidr,198.51.100.0/24",
					},
				},
			},
			expectedSources: []string{"203.0.113.0/24", "198.51.100.0/24"},
		},
		{
			name: "no sources configured",
			service: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{},
				},
			},
			expectedSources: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expectedSources, lb.getIcmpAllowedSources(tt.service))
		})
	}
}

func TestBuildIcmpIngressRules(t *testing.T) {
	lb := &loadbalancer{}

	tests := []struct {
		name          string
		service       *corev1.Service
		expectedRules []iaas.SecurityGroupRule
	}{
		{
			name: "icmp disabled returns no rules",
			service: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "svc",
					Namespace: "default",
					Annotations: map[string]string{
						LoadbalancerAnnotationAclAllowedSources: "10.0.0.0/8",
					},
				},
			},
			expectedRules: nil,
		},
		{
			name: "icmp enabled with custom sources",
			service: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "svc",
					Namespace: "default",
					Annotations: map[string]string{
						LoadBalancerAnnotationSecurityGroupAllowICMP:          "true",
						LoadBalancerAnnotationSecurityGroupICMPAllowedSources: "203.0.113.0/24,2001:db8::/32",
					},
				},
			},
			expectedRules: []iaas.SecurityGroupRule{
				{
					Name:          "allow-icmp",
					IPVersion:     iaas.SecurityGroupIPVersionIPv4,
					Protocol:      iaas.SecurityGroupRuleProtocolICMP,
					Priority:      190,
					RemoteType:    iaas.SecurityGroupRuleRemoteTypeAddress,
					RemoteAddress: ptr.To("203.0.113.0/24"),
					PortRangeMin:  1,
					PortRangeMax:  1,
					Policy:        iaas.SecurityGroupRulePolicyAllow,
				},
				{
					Name:          "allow-icmp",
					IPVersion:     iaas.SecurityGroupIPVersionIPv6,
					Protocol:      iaas.SecurityGroupRuleProtocolICMP,
					Priority:      191,
					RemoteType:    iaas.SecurityGroupRuleRemoteTypeAddress,
					RemoteAddress: ptr.To("2001:db8::/32"),
					PortRangeMin:  1,
					PortRangeMax:  1,
					Policy:        iaas.SecurityGroupRulePolicyAllow,
				},
			},
		},
		{
			name: "icmp enabled inherits acl sources",
			service: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "svc",
					Namespace: "default",
					Annotations: map[string]string{
						LoadBalancerAnnotationSecurityGroupAllowICMP: "true",
						LoadbalancerAnnotationAclAllowedSources:      "10.0.0.0/8",
					},
				},
			},
			expectedRules: []iaas.SecurityGroupRule{
				{
					Name:          "allow-icmp",
					IPVersion:     iaas.SecurityGroupIPVersionIPv4,
					Protocol:      iaas.SecurityGroupRuleProtocolICMP,
					Priority:      190,
					RemoteType:    iaas.SecurityGroupRuleRemoteTypeAddress,
					RemoteAddress: ptr.To("10.0.0.0/8"),
					PortRangeMin:  1,
					PortRangeMax:  1,
					Policy:        iaas.SecurityGroupRulePolicyAllow,
				},
			},
		},
		{
			name: "icmp enabled without sources fails closed",
			service: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "svc",
					Namespace: "default",
					Annotations: map[string]string{
						LoadBalancerAnnotationSecurityGroupAllowICMP: "true",
					},
				},
			},
			expectedRules: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rules := lb.buildIcmpIngressRules(tt.service)
			require.Equal(t, len(tt.expectedRules), len(rules))
			for i := range tt.expectedRules {
				assert.Equal(t, tt.expectedRules[i], rules[i])
			}
		})
	}
}

func TestSecurityGroupIPVersionForCIDR(t *testing.T) {
	tests := []struct {
		name     string
		cidr     string
		expected iaas.SecurityGroupIPVersion
	}{
		{name: "ipv4", cidr: "10.0.0.0/8", expected: iaas.SecurityGroupIPVersionIPv4},
		{name: "ipv6", cidr: "2001:db8::/32", expected: iaas.SecurityGroupIPVersionIPv6},
		{name: "invalid defaults to ipv4", cidr: "not-a-cidr", expected: iaas.SecurityGroupIPVersionIPv4},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, securityGroupIPVersionForCIDR(tt.cidr))
		})
	}
}

func TestBuildEgressRules(t *testing.T) {
	lb := &loadbalancer{}
	rules := lb.buildEgressRules()

	require.Len(t, rules, 2)
	for i, rule := range rules {
		assert.Equal(t, iaas.SecurityGroupRulePolicyAllow, rule.Policy, "rule %d must set policy", i)
		assert.Equal(t, iaas.SecurityGroupRuleProtocolAll, rule.Protocol)
		assert.Equal(t, iaas.SecurityGroupRuleRemoteTypeAddress, rule.RemoteType)
		assert.Greater(t, rule.PortRangeMin, int32(0))
		assert.Greater(t, rule.PortRangeMax, int32(0))
		assert.Less(t, rule.PortRangeMax, int32(65535))
		assert.NotEmpty(t, rule.Name)
		require.NotNil(t, rule.RemoteAddress)
	}

	assert.Equal(t, iaas.SecurityGroupIPVersionIPv4, rules[0].IPVersion)
	assert.Equal(t, "0.0.0.0/0", *rules[0].RemoteAddress)
	assert.Equal(t, iaas.SecurityGroupIPVersionIPv6, rules[1].IPVersion)
	assert.Equal(t, "::/0", *rules[1].RemoteAddress)
	assert.NotEqual(t, rules[0].Name, rules[1].Name)
	assert.NotEqual(t, rules[0].Priority, rules[1].Priority)
}

func TestBuildIngressRulesFromListeners_UniquePriorities(t *testing.T) {
	lb := &loadbalancer{}
	listeners := []iaas.VpcLoadbalancerListener{
		{Protocol: iaas.ProtocolTCP, Port: 80, AllowedSources: []string{"0.0.0.0/0"}},
		{Protocol: iaas.ProtocolTCP, Port: 443, AllowedSources: []string{"0.0.0.0/0"}},
		{Protocol: iaas.ProtocolTCP, Port: 22, AllowedSources: []string{"0.0.0.0/0"}},
	}

	rules := lb.buildIngressRulesFromListeners(listeners)
	require.Len(t, rules, 3)

	seen := map[int32]bool{}
	for _, rule := range rules {
		assert.False(t, seen[rule.Priority], "duplicate priority %d", rule.Priority)
		seen[rule.Priority] = true
		assert.Equal(t, iaas.SecurityGroupRulePolicyAllow, rule.Policy)
	}
}
