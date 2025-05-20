package provider

import (
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestGetStringAnnotation(t *testing.T) {
	tests := []struct {
		name         string
		service      *corev1.Service
		annotation   string
		defaultValue string
		expected     string
		expectError  bool
	}{
		{
			name: "loadbalancer type annotation exists",
			service: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						LoadBalancerAnnotationLoadbalancerType: "public",
					},
				},
			},
			annotation:   LoadBalancerAnnotationLoadbalancerType,
			defaultValue: "private",
			expected:     "public",
			expectError:  false,
		},
		{
			name: "subnet ID annotation exists",
			service: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						LoadBalancerAnnotationSubnetID: "subnet-123",
					},
				},
			},
			annotation:   LoadBalancerAnnotationSubnetID,
			defaultValue: "default-subnet",
			expected:     "subnet-123",
			expectError:  false,
		},
		{
			name: "loadbalancing policy annotation exists",
			service: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						LoadbalancerAnnotationLoadbalancingPolicy: "MAGLEV",
					},
				},
			},
			annotation:   LoadbalancerAnnotationLoadbalancingPolicy,
			defaultValue: DefaultLoadbalancingPolicy,
			expected:     "MAGLEV",
			expectError:  false,
		},
		{
			name: "annotation does not exist",
			service: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{},
				},
			},
			annotation:   LoadBalancerAnnotationLoadbalancerType,
			defaultValue: "private",
			expected:     "private",
			expectError:  false,
		},
		{
			name: "nil annotations",
			service: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{},
			},
			annotation:   LoadBalancerAnnotationLoadbalancerType,
			defaultValue: "private",
			expected:     "private",
			expectError:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := getStringAnnotation(tt.service, tt.annotation, tt.defaultValue)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestGetIntAnnotation(t *testing.T) {
	tests := []struct {
		name         string
		service      *corev1.Service
		annotation   string
		defaultValue int
		expected     int
		expectError  bool
	}{
		{
			name: "valid idle connection timeout",
			service: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						LoadbalancerAnnotationIdleConnectionTimeout: "3000",
					},
				},
			},
			annotation:   LoadbalancerAnnotationIdleConnectionTimeout,
			defaultValue: DefaultIdleConnectionTimeout,
			expected:     3000,
			expectError:  false,
		},
		{
			name: "valid max connections",
			service: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						LoadbalancerAnnotationMaxConnections: "5000",
					},
				},
			},
			annotation:   LoadbalancerAnnotationMaxConnections,
			defaultValue: DefaultMaxConnections,
			expected:     5000,
			expectError:  false,
		},
		{
			name: "invalid health check port",
			service: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						LoadbalancerAnnotationHealthCheckPort: "not-a-number",
					},
				},
			},
			annotation:   LoadbalancerAnnotationHealthCheckPort,
			defaultValue: 8080,
			expected:     8080,
			expectError:  true,
		},
		{
			name: "annotation does not exist",
			service: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{},
				},
			},
			annotation:   LoadbalancerAnnotationIdleConnectionTimeout,
			defaultValue: DefaultIdleConnectionTimeout,
			expected:     DefaultIdleConnectionTimeout,
			expectError:  false,
		},
		{
			name: "nil annotations",
			service: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{},
			},
			annotation:   LoadbalancerAnnotationIdleConnectionTimeout,
			defaultValue: DefaultIdleConnectionTimeout,
			expected:     DefaultIdleConnectionTimeout,
			expectError:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := getIntAnnotation(tt.service, tt.annotation, tt.defaultValue)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestGetBoolAnnotation(t *testing.T) {
	tests := []struct {
		name         string
		service      *corev1.Service
		annotation   string
		defaultValue bool
		expected     bool
		expectError  bool
	}{
		{
			name: "valid internal loadbalancer annotation",
			service: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						LoadbalancerAnnotationInternal: "true",
					},
				},
			},
			annotation:   LoadbalancerAnnotationInternal,
			defaultValue: false,
			expected:     true,
			expectError:  false,
		},
		{
			name: "valid proxy protocol annotation",
			service: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						LoadbalancerAnnotationEnableProxyProtocol: "true",
					},
				},
			},
			annotation:   LoadbalancerAnnotationEnableProxyProtocol,
			defaultValue: DefaultEnableProxyProtocol,
			expected:     true,
			expectError:  false,
		},
		{
			name: "valid health check enabled annotation",
			service: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						LoadbalancerAnnotationHealthCheckEnabled: "false",
					},
				},
			},
			annotation:   LoadbalancerAnnotationHealthCheckEnabled,
			defaultValue: true,
			expected:     false,
			expectError:  false,
		},
		{
			name: "invalid boolean annotation",
			service: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						LoadbalancerAnnotationInternal: "not-a-bool",
					},
				},
			},
			annotation:   LoadbalancerAnnotationInternal,
			defaultValue: false,
			expected:     false,
			expectError:  true,
		},
		{
			name: "annotation does not exist",
			service: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{},
				},
			},
			annotation:   LoadbalancerAnnotationInternal,
			defaultValue: false,
			expected:     false,
			expectError:  false,
		},
		{
			name: "nil annotations",
			service: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{},
			},
			annotation:   LoadbalancerAnnotationInternal,
			defaultValue: false,
			expected:     false,
			expectError:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := getBoolAnnotation(tt.service, tt.annotation, tt.defaultValue)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}
