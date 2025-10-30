package provider

import (
	"testing"

	"github.com/thalassa-cloud/client-go/iaas"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetPerPortAclAllowedSources(t *testing.T) {
	lb := &loadbalancer{}

	tests := []struct {
		name            string
		service         *corev1.Service
		port            corev1.ServicePort
		expectedSources []string
	}{
		{
			name: "port with name annotation",
			service: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"loadbalancer.k8s.thalassa.cloud/acl-port-http": "10.0.0.0/8,192.168.1.0/24",
					},
				},
			},
			port: corev1.ServicePort{
				Name: "http",
				Port: 80,
			},
			expectedSources: []string{"10.0.0.0/8", "192.168.1.0/24"},
		},
		{
			name: "port with number annotation",
			service: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"loadbalancer.k8s.thalassa.cloud/acl-port-443": "172.16.0.0/12",
					},
				},
			},
			port: corev1.ServicePort{
				Name: "https",
				Port: 443,
			},
			expectedSources: []string{"172.16.0.0/12"},
		},
		{
			name: "port with both name and number annotations",
			service: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"loadbalancer.k8s.thalassa.cloud/acl-port-http": "10.0.0.0/8",
						"loadbalancer.k8s.thalassa.cloud/acl-port-80":   "192.168.1.0/24",
					},
				},
			},
			port: corev1.ServicePort{
				Name: "http",
				Port: 80,
			},
			expectedSources: []string{"10.0.0.0/8", "192.168.1.0/24"},
		},
		{
			name: "port with no annotations",
			service: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{},
				},
			},
			port: corev1.ServicePort{
				Name: "http",
				Port: 80,
			},
			expectedSources: []string{},
		},
		{
			name: "port with invalid CIDR in annotation",
			service: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"loadbalancer.k8s.thalassa.cloud/acl-port-http": "10.0.0.0/8,invalid-cidr,192.168.1.0/24",
					},
				},
			},
			port: corev1.ServicePort{
				Name: "http",
				Port: 80,
			},
			expectedSources: []string{"10.0.0.0/8", "192.168.1.0/24"},
		},
		{
			name: "port with empty annotation value",
			service: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"loadbalancer.k8s.thalassa.cloud/acl-port-http": "",
					},
				},
			},
			port: corev1.ServicePort{
				Name: "http",
				Port: 80,
			},
			expectedSources: []string{},
		},
		{
			name: "port with whitespace in annotation value",
			service: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"loadbalancer.k8s.thalassa.cloud/acl-port-http": " 10.0.0.0/8 , 192.168.1.0/24 ",
					},
				},
			},
			port: corev1.ServicePort{
				Name: "http",
				Port: 80,
			},
			expectedSources: []string{"10.0.0.0/8", "192.168.1.0/24"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := lb.getPerPortAclAllowedSources(tt.service, tt.port)
			assert.Equal(t, tt.expectedSources, result)
		})
	}
}

func TestParseAclSources(t *testing.T) {
	lb := &loadbalancer{}

	tests := []struct {
		name            string
		input           string
		expectedSources []string
	}{
		{
			name:            "valid CIDR ranges",
			input:           "10.0.0.0/8,192.168.1.0/24,172.16.0.0/12",
			expectedSources: []string{"10.0.0.0/8", "192.168.1.0/24", "172.16.0.0/12"},
		},
		{
			name:            "mixed valid and invalid CIDR ranges",
			input:           "10.0.0.0/8,invalid-cidr,192.168.1.0/24,another-invalid",
			expectedSources: []string{"10.0.0.0/8", "192.168.1.0/24"},
		},
		{
			name:            "empty string",
			input:           "",
			expectedSources: []string{},
		},
		{
			name:            "whitespace only",
			input:           "   ,  ,  ",
			expectedSources: []string{},
		},
		{
			name:            "single valid CIDR",
			input:           "10.0.0.0/8",
			expectedSources: []string{"10.0.0.0/8"},
		},
		{
			name:            "whitespace around CIDRs",
			input:           " 10.0.0.0/8 , 192.168.1.0/24 ",
			expectedSources: []string{"10.0.0.0/8", "192.168.1.0/24"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := lb.parseAclSources(tt.input)
			assert.Equal(t, tt.expectedSources, result)
		})
	}
}

func TestRemoveDuplicateStrings(t *testing.T) {
	lb := &loadbalancer{}

	tests := []struct {
		name     string
		input    []string
		expected []string
	}{
		{
			name:     "no duplicates",
			input:    []string{"10.0.0.0/8", "192.168.1.0/24", "172.16.0.0/12"},
			expected: []string{"10.0.0.0/8", "192.168.1.0/24", "172.16.0.0/12"},
		},
		{
			name:     "with duplicates",
			input:    []string{"10.0.0.0/8", "192.168.1.0/24", "10.0.0.0/8", "172.16.0.0/12", "192.168.1.0/24"},
			expected: []string{"10.0.0.0/8", "192.168.1.0/24", "172.16.0.0/12"},
		},
		{
			name:     "empty slice",
			input:    []string{},
			expected: []string{},
		},
		{
			name:     "single element",
			input:    []string{"10.0.0.0/8"},
			expected: []string{"10.0.0.0/8"},
		},
		{
			name:     "all duplicates",
			input:    []string{"10.0.0.0/8", "10.0.0.0/8", "10.0.0.0/8"},
			expected: []string{"10.0.0.0/8"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := lb.removeDuplicateStrings(tt.input)
			assert.ElementsMatch(t, tt.expected, result)
		})
	}
}

func TestDesiredVpcLoadbalancerListener_WithPerPortAcl(t *testing.T) {
	lb := &loadbalancer{
		cluster: "test-cluster",
	}

	tests := []struct {
		name              string
		service           *corev1.Service
		expectedListeners []iaas.VpcLoadbalancerListener
	}{
		{
			name: "service with global ACL only",
			service: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-service",
					Namespace: "default",
					Annotations: map[string]string{
						"loadbalancer.k8s.thalassa.cloud/acl-allowed-sources": "10.0.0.0/8,192.168.1.0/24",
					},
				},
				Spec: corev1.ServiceSpec{
					Ports: []corev1.ServicePort{
						{
							Name:     "http",
							Port:     80,
							Protocol: corev1.ProtocolTCP,
						},
						{
							Name:     "https",
							Port:     443,
							Protocol: corev1.ProtocolTCP,
						},
					},
				},
			},
			expectedListeners: []iaas.VpcLoadbalancerListener{
				{
					Port:           80,
					Protocol:       iaas.ProtocolTCP,
					AllowedSources: []string{"10.0.0.0/8", "192.168.1.0/24"},
				},
				{
					Port:           443,
					Protocol:       iaas.ProtocolTCP,
					AllowedSources: []string{"10.0.0.0/8", "192.168.1.0/24"},
				},
			},
		},
		{
			name: "service with per-port ACL only",
			service: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-service",
					Namespace: "default",
					Annotations: map[string]string{
						"loadbalancer.k8s.thalassa.cloud/acl-port-http": "10.0.0.0/8",
						"loadbalancer.k8s.thalassa.cloud/acl-port-443":  "172.16.0.0/12",
					},
				},
				Spec: corev1.ServiceSpec{
					Ports: []corev1.ServicePort{
						{
							Name:     "http",
							Port:     80,
							Protocol: corev1.ProtocolTCP,
						},
						{
							Name:     "https",
							Port:     443,
							Protocol: corev1.ProtocolTCP,
						},
					},
				},
			},
			expectedListeners: []iaas.VpcLoadbalancerListener{
				{
					Port:           80,
					Protocol:       iaas.ProtocolTCP,
					AllowedSources: []string{"10.0.0.0/8"},
				},
				{
					Port:           443,
					Protocol:       iaas.ProtocolTCP,
					AllowedSources: []string{"172.16.0.0/12"},
				},
			},
		},
		{
			name: "service with global and per-port ACL combined",
			service: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-service",
					Namespace: "default",
					Annotations: map[string]string{
						"loadbalancer.k8s.thalassa.cloud/acl-allowed-sources": "10.0.0.0/8,192.168.1.0/24",
						"loadbalancer.k8s.thalassa.cloud/acl-port-http":       "172.16.0.0/12",
						"loadbalancer.k8s.thalassa.cloud/acl-port-443":        "10.10.0.0/16",
					},
				},
				Spec: corev1.ServiceSpec{
					Ports: []corev1.ServicePort{
						{
							Name:     "http",
							Port:     80,
							Protocol: corev1.ProtocolTCP,
						},
						{
							Name:     "https",
							Port:     443,
							Protocol: corev1.ProtocolTCP,
						},
					},
				},
			},
			expectedListeners: []iaas.VpcLoadbalancerListener{
				{
					Port:           80,
					Protocol:       iaas.ProtocolTCP,
					AllowedSources: []string{"10.0.0.0/8", "192.168.1.0/24", "172.16.0.0/12"},
				},
				{
					Port:           443,
					Protocol:       iaas.ProtocolTCP,
					AllowedSources: []string{"10.0.0.0/8", "192.168.1.0/24", "10.10.0.0/16"},
				},
			},
		},
		{
			name: "service with no ACL annotations",
			service: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "test-service",
					Namespace:   "default",
					Annotations: map[string]string{},
				},
				Spec: corev1.ServiceSpec{
					Ports: []corev1.ServicePort{
						{
							Name:     "http",
							Port:     80,
							Protocol: corev1.ProtocolTCP,
						},
					},
				},
			},
			expectedListeners: []iaas.VpcLoadbalancerListener{
				{
					Port:           80,
					Protocol:       iaas.ProtocolTCP,
					AllowedSources: []string{},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			listeners := lb.desiredVpcLoadbalancerListener(tt.service)

			require.Len(t, listeners, len(tt.expectedListeners))

			for i, expected := range tt.expectedListeners {
				assert.Equal(t, expected.Port, listeners[i].Port)
				assert.Equal(t, expected.Protocol, listeners[i].Protocol)

				assert.ElementsMatch(t, listeners[i].AllowedSources, expected.AllowedSources)
			}
		})
	}
}
