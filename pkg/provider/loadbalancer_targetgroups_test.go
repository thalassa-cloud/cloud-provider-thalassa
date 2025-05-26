package provider

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/thalassa-cloud/client-go/iaas"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

func TestGetDesiredVpcLoadbalancerTargetGroups(t *testing.T) {
	tests := []struct {
		name          string
		service       *corev1.Service
		expectedTGs   []iaas.VpcLoadbalancerTargetGroup
		expectedError bool
	}{
		{
			name: "basic target group with default values",
			service: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-service",
					Namespace: "default",
					UID:       "test-uid-1",
				},
				Spec: corev1.ServiceSpec{
					Ports: []corev1.ServicePort{
						{
							Name:     "http",
							Protocol: corev1.ProtocolTCP,
							Port:     80,
							NodePort: 30000,
						},
					},
				},
			},
			expectedTGs: []iaas.VpcLoadbalancerTargetGroup{
				{
					Name:                "atestuid1-http",
					TargetPort:          30000,
					Protocol:            iaas.ProtocolTCP,
					EnableProxyProtocol: ptr.To(false),
					LoadbalancingPolicy: ptr.To(iaas.LoadbalancingPolicyRoundRobin),
				},
			},
			expectedError: false,
		},
		{
			name: "target group with health check enabled",
			service: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-service",
					Namespace: "default",
					UID:       "test-uid-2",
					Annotations: map[string]string{
						LoadbalancerAnnotationHealthCheckEnabled: "true",
						LoadbalancerAnnotationHealthCheckPort:    "8080",
					},
				},
				Spec: corev1.ServiceSpec{
					Ports: []corev1.ServicePort{
						{
							Name:     "http",
							Protocol: corev1.ProtocolTCP,
							Port:     80,
							NodePort: 30000,
						},
					},
				},
			},
			expectedTGs: []iaas.VpcLoadbalancerTargetGroup{
				{
					Name:                "atestuid2-http",
					TargetPort:          30000,
					Protocol:            iaas.ProtocolTCP,
					EnableProxyProtocol: ptr.To(false),
					LoadbalancingPolicy: ptr.To(iaas.LoadbalancingPolicyRoundRobin),
					HealthCheck: &iaas.BackendHealthCheck{
						Port:               8080,
						Protocol:           iaas.ProtocolHTTP,
						Path:               DefaultHealthCheckPath,
						TimeoutSeconds:     DefaultHealthCheckTimeoutSeconds,
						PeriodSeconds:      DefaultHealthCheckPeriodSeconds,
						HealthyThreshold:   DefaultHealthCheckHealthyThreshold,
						UnhealthyThreshold: DefaultHealthCheckUnhealthyThreshold,
					},
				},
			},
			expectedError: false,
		},
		{
			name: "target group with custom health check configuration",
			service: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-service",
					Namespace: "default",
					UID:       "test-uid-3",
					Annotations: map[string]string{
						LoadbalancerAnnotationHealthCheckEnabled:       "true",
						LoadbalancerAnnotationHealthCheckPort:          "8080",
						LoadbalancerAnnotationHealthCheckPath:          "/custom-health",
						LoadbalancerAnnotationHealthCheckTimeout:       "10",
						LoadbalancerAnnotationHealthCheckInterval:      "20",
						LoadbalancerAnnotationHealthCheckUpThreshold:   "3",
						LoadbalancerAnnotationHealthCheckDownThreshold: "4",
					},
				},
				Spec: corev1.ServiceSpec{
					Ports: []corev1.ServicePort{
						{
							Name:     "http",
							Protocol: corev1.ProtocolTCP,
							Port:     80,
							NodePort: 30000,
						},
					},
				},
			},
			expectedTGs: []iaas.VpcLoadbalancerTargetGroup{
				{
					Name:                "atestuid3-http",
					TargetPort:          30000,
					Protocol:            iaas.ProtocolTCP,
					EnableProxyProtocol: ptr.To(false),
					LoadbalancingPolicy: ptr.To(iaas.LoadbalancingPolicyRoundRobin),
					HealthCheck: &iaas.BackendHealthCheck{
						Port:               8080,
						Protocol:           iaas.ProtocolHTTP,
						Path:               "/custom-health",
						TimeoutSeconds:     10,
						PeriodSeconds:      20,
						HealthyThreshold:   3,
						UnhealthyThreshold: 4,
					},
				},
			},
			expectedError: false,
		},
		{
			name: "target group with proxy protocol enabled",
			service: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-service",
					Namespace: "default",
					UID:       "test-uid-4",
					Annotations: map[string]string{
						LoadbalancerAnnotationEnableProxyProtocol: "true",
					},
				},
				Spec: corev1.ServiceSpec{
					Ports: []corev1.ServicePort{
						{
							Name:     "http",
							Protocol: corev1.ProtocolTCP,
							Port:     80,
							NodePort: 30000,
						},
					},
				},
			},
			expectedTGs: []iaas.VpcLoadbalancerTargetGroup{
				{
					Name:                "atestuid4-http",
					TargetPort:          30000,
					Protocol:            iaas.ProtocolTCP,
					EnableProxyProtocol: ptr.To(true),
					LoadbalancingPolicy: ptr.To(iaas.LoadbalancingPolicyRoundRobin),
				},
			},
			expectedError: false,
		},
		{
			name: "target group with custom loadbalancing policy",
			service: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-service",
					Namespace: "default",
					UID:       "test-uid-5",
					Annotations: map[string]string{
						LoadbalancerAnnotationLoadbalancingPolicy: "MAGLEV",
					},
				},
				Spec: corev1.ServiceSpec{
					Ports: []corev1.ServicePort{
						{
							Name:     "http",
							Protocol: corev1.ProtocolTCP,
							Port:     80,
							NodePort: 30000,
						},
					},
				},
			},
			expectedTGs: []iaas.VpcLoadbalancerTargetGroup{
				{
					Name:                "atestuid5-http",
					TargetPort:          30000,
					Protocol:            iaas.ProtocolTCP,
					EnableProxyProtocol: ptr.To(false),
					LoadbalancingPolicy: ptr.To(iaas.LoadbalancingPolicyMagLev),
				},
			},
			expectedError: false,
		},
		{
			name: "service with multiple ports",
			service: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-service",
					Namespace: "default",
					UID:       "test-uid-6",
				},
				Spec: corev1.ServiceSpec{
					Ports: []corev1.ServicePort{
						{
							Name:     "http",
							Protocol: corev1.ProtocolTCP,
							Port:     80,
							NodePort: 30000,
						},
						{
							Name:     "https",
							Protocol: corev1.ProtocolTCP,
							Port:     443,
							NodePort: 30001,
						},
					},
				},
			},
			expectedTGs: []iaas.VpcLoadbalancerTargetGroup{
				{
					Name:                "atestuid6-http",
					TargetPort:          30000,
					Protocol:            iaas.ProtocolTCP,
					EnableProxyProtocol: ptr.To(false),
					LoadbalancingPolicy: ptr.To(iaas.LoadbalancingPolicyRoundRobin),
				},
				{
					Name:                "atestuid6-https",
					TargetPort:          30001,
					Protocol:            iaas.ProtocolTCP,
					EnableProxyProtocol: ptr.To(false),
					LoadbalancingPolicy: ptr.To(iaas.LoadbalancingPolicyRoundRobin),
				},
			},
			expectedError: false,
		},
		{
			name: "service with HealthCheckNodePort for local traffic policy",
			service: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-service",
					Namespace: "default",
					UID:       "test-uid-7",
				},
				Spec: corev1.ServiceSpec{
					ExternalTrafficPolicy: corev1.ServiceExternalTrafficPolicyLocal,
					HealthCheckNodePort:   31000,
					Ports: []corev1.ServicePort{
						{
							Name:     "http",
							Protocol: corev1.ProtocolTCP,
							Port:     80,
							NodePort: 30000,
						},
					},
				},
			},
			expectedTGs: []iaas.VpcLoadbalancerTargetGroup{
				{
					Name:                "atestuid7-http",
					TargetPort:          30000,
					Protocol:            iaas.ProtocolTCP,
					EnableProxyProtocol: ptr.To(false),
					LoadbalancingPolicy: ptr.To(iaas.LoadbalancingPolicyRoundRobin),
					HealthCheck: &iaas.BackendHealthCheck{
						Port:               31000,
						Protocol:           iaas.ProtocolHTTP,
						Path:               DefaultHealthCheckPath,
						TimeoutSeconds:     DefaultHealthCheckTimeoutSeconds,
						PeriodSeconds:      DefaultHealthCheckPeriodSeconds,
						HealthyThreshold:   DefaultHealthCheckHealthyThreshold,
						UnhealthyThreshold: DefaultHealthCheckUnhealthyThreshold,
					},
				},
			},
			expectedError: false,
		},
		{
			name: "service with HealthCheckNodePort and custom health check port",
			service: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-service",
					Namespace: "default",
					UID:       "test-uid-8",
					Annotations: map[string]string{
						LoadbalancerAnnotationHealthCheckPort: "8080",
					},
				},
				Spec: corev1.ServiceSpec{
					ExternalTrafficPolicy: corev1.ServiceExternalTrafficPolicyLocal,
					HealthCheckNodePort:   31000,
					Ports: []corev1.ServicePort{
						{
							Name:     "http",
							Protocol: corev1.ProtocolTCP,
							Port:     80,
							NodePort: 30000,
						},
					},
				},
			},
			expectedTGs: []iaas.VpcLoadbalancerTargetGroup{
				{
					Name:                "atestuid8-http",
					TargetPort:          30000,
					Protocol:            iaas.ProtocolTCP,
					EnableProxyProtocol: ptr.To(false),
					LoadbalancingPolicy: ptr.To(iaas.LoadbalancingPolicyRoundRobin),
					HealthCheck: &iaas.BackendHealthCheck{
						Port:               8080,
						Protocol:           iaas.ProtocolHTTP,
						Path:               DefaultHealthCheckPath,
						TimeoutSeconds:     DefaultHealthCheckTimeoutSeconds,
						PeriodSeconds:      DefaultHealthCheckPeriodSeconds,
						HealthyThreshold:   DefaultHealthCheckHealthyThreshold,
						UnhealthyThreshold: DefaultHealthCheckUnhealthyThreshold,
					},
				},
			},
			expectedError: false,
		},
		{
			name: "service with invalid health check timeout",
			service: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-service",
					Namespace: "default",
					UID:       "test-uid-9",
					Annotations: map[string]string{
						LoadbalancerAnnotationHealthCheckEnabled: "true",
						LoadbalancerAnnotationHealthCheckPort:    "8080",
						LoadbalancerAnnotationHealthCheckTimeout: "invalid",
					},
				},
				Spec: corev1.ServiceSpec{
					Ports: []corev1.ServicePort{
						{
							Name:     "http",
							Protocol: corev1.ProtocolTCP,
							Port:     80,
							NodePort: 30000,
						},
					},
				},
			},
			expectedTGs: []iaas.VpcLoadbalancerTargetGroup{
				{
					Name:                "atestuid9-http",
					TargetPort:          30000,
					Protocol:            iaas.ProtocolTCP,
					EnableProxyProtocol: ptr.To(false),
					LoadbalancingPolicy: ptr.To(iaas.LoadbalancingPolicyRoundRobin),
					HealthCheck: &iaas.BackendHealthCheck{
						Port:               8080,
						Protocol:           iaas.ProtocolHTTP,
						Path:               DefaultHealthCheckPath,
						TimeoutSeconds:     DefaultHealthCheckTimeoutSeconds,
						PeriodSeconds:      DefaultHealthCheckPeriodSeconds,
						HealthyThreshold:   DefaultHealthCheckHealthyThreshold,
						UnhealthyThreshold: DefaultHealthCheckUnhealthyThreshold,
					},
				},
			},
			expectedError: false,
		},
		{
			name: "service with invalid loadbalancing policy",
			service: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-service",
					Namespace: "default",
					UID:       "test-uid-10",
					Annotations: map[string]string{
						LoadbalancerAnnotationLoadbalancingPolicy: "INVALID_POLICY",
					},
				},
				Spec: corev1.ServiceSpec{
					Ports: []corev1.ServicePort{
						{
							Name:     "http",
							Protocol: corev1.ProtocolTCP,
							Port:     80,
							NodePort: 30000,
						},
					},
				},
			},
			expectedTGs:   nil,
			expectedError: true,
		},
		{
			name: "service with invalid proxy protocol value",
			service: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-service",
					Namespace: "default",
					UID:       "test-uid-11",
					Annotations: map[string]string{
						LoadbalancerAnnotationEnableProxyProtocol: "invalid",
					},
				},
				Spec: corev1.ServiceSpec{
					Ports: []corev1.ServicePort{
						{
							Name:     "http",
							Protocol: corev1.ProtocolTCP,
							Port:     80,
							NodePort: 30000,
						},
					},
				},
			},
			expectedTGs: []iaas.VpcLoadbalancerTargetGroup{
				{
					Name:                "atestuid11-http",
					TargetPort:          30000,
					Protocol:            iaas.ProtocolTCP,
					EnableProxyProtocol: ptr.To(false),
					LoadbalancingPolicy: ptr.To(iaas.LoadbalancingPolicyRoundRobin),
				},
			},
			expectedError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lb := &loadbalancer{}
			tgs, err := lb.getDesiredVpcLoadbalancerTargetGroups(tt.service, nil)

			if tt.expectedError {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, len(tt.expectedTGs), len(tgs))

			for i, expectedTG := range tt.expectedTGs {
				assert.Equal(t, expectedTG.Name, tgs[i].Name)
				assert.Equal(t, expectedTG.TargetPort, tgs[i].TargetPort)
				assert.Equal(t, expectedTG.Protocol, tgs[i].Protocol)
				assert.Equal(t, expectedTG.EnableProxyProtocol, tgs[i].EnableProxyProtocol)
				assert.Equal(t, expectedTG.LoadbalancingPolicy, tgs[i].LoadbalancingPolicy)

				if expectedTG.HealthCheck != nil {
					assert.NotNil(t, tgs[i].HealthCheck)
					assert.Equal(t, expectedTG.HealthCheck.Port, tgs[i].HealthCheck.Port)
					assert.Equal(t, expectedTG.HealthCheck.Protocol, tgs[i].HealthCheck.Protocol)
					assert.Equal(t, expectedTG.HealthCheck.Path, tgs[i].HealthCheck.Path)
					assert.Equal(t, expectedTG.HealthCheck.TimeoutSeconds, tgs[i].HealthCheck.TimeoutSeconds)
					assert.Equal(t, expectedTG.HealthCheck.PeriodSeconds, tgs[i].HealthCheck.PeriodSeconds)
					assert.Equal(t, expectedTG.HealthCheck.HealthyThreshold, tgs[i].HealthCheck.HealthyThreshold)
					assert.Equal(t, expectedTG.HealthCheck.UnhealthyThreshold, tgs[i].HealthCheck.UnhealthyThreshold)
				} else {
					assert.Nil(t, tgs[i].HealthCheck)
				}
			}
		})
	}
}
