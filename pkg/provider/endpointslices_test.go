package provider

import (
	"context"
	"sync"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/klog/v2"
	"k8s.io/utils/ptr"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEndpointSliceWatcher_NodeAssignmentChanges(t *testing.T) {
	// Create a fake Kubernetes client
	client := fake.NewSimpleClientset()

	// Track resync calls
	var resyncCalls []string
	var resyncMutex sync.Mutex

	resyncCallback := func(serviceKey string) {
		resyncMutex.Lock()
		defer resyncMutex.Unlock()
		resyncCalls = append(resyncCalls, serviceKey)
		klog.Infof("Resync triggered for service: %s", serviceKey)
	}

	// Create stop channel
	stopCh := make(chan struct{})
	defer close(stopCh)

	// Create the endpoint slice watcher
	_ = NewEndpointSliceWatcher(client, stopCh, resyncCallback)

	// Create a service with externalTrafficPolicy=Local
	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-service",
			Namespace: "default",
		},
		Spec: corev1.ServiceSpec{
			Type:                  corev1.ServiceTypeLoadBalancer,
			ExternalTrafficPolicy: corev1.ServiceExternalTrafficPolicyTypeLocal,
		},
	}

	_, err := client.CoreV1().Services("default").Create(context.Background(), service, metav1.CreateOptions{})
	require.NoError(t, err)

	// Wait a bit for the informer to process the service
	time.Sleep(100 * time.Millisecond)

	// Create initial endpoint slice with pods on node-1
	initialEpSlice := &discoveryv1.EndpointSlice{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-service-abc123",
			Namespace: "default",
			Labels: map[string]string{
				discoveryv1.LabelServiceName: "test-service",
			},
		},
		Endpoints: []discoveryv1.Endpoint{
			{
				NodeName: ptr.To("node-1"),
				Conditions: discoveryv1.EndpointConditions{
					Ready: ptr.To(true),
				},
			},
		},
	}

	_, err = client.DiscoveryV1().EndpointSlices("default").Create(context.Background(), initialEpSlice, metav1.CreateOptions{})
	require.NoError(t, err)

	// Wait a bit for the informer to process the endpoint slice
	time.Sleep(100 * time.Millisecond)

	// Update endpoint slice to move pods to node-2
	updatedEpSlice := &discoveryv1.EndpointSlice{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-service-abc123",
			Namespace: "default",
			Labels: map[string]string{
				discoveryv1.LabelServiceName: "test-service",
			},
		},
		Endpoints: []discoveryv1.Endpoint{
			{
				NodeName: ptr.To("node-2"),
				Conditions: discoveryv1.EndpointConditions{
					Ready: ptr.To(true),
				},
			},
		},
	}

	_, err = client.DiscoveryV1().EndpointSlices("default").Update(context.Background(), updatedEpSlice, metav1.UpdateOptions{})
	require.NoError(t, err)

	// Wait for the resync to be triggered
	time.Sleep(200 * time.Millisecond)

	// Check that resync was triggered
	resyncMutex.Lock()
	defer resyncMutex.Unlock()

	assert.Contains(t, resyncCalls, "default/test-service", "Expected resync to be triggered for the service")
}

func TestEndpointSliceWatcher_ExternalTrafficPolicyChanges(t *testing.T) {
	// Create a fake Kubernetes client
	client := fake.NewSimpleClientset()

	// Track resync calls
	var resyncCalls []string
	var resyncMutex sync.Mutex

	resyncCallback := func(serviceKey string) {
		resyncMutex.Lock()
		defer resyncMutex.Unlock()
		resyncCalls = append(resyncCalls, serviceKey)
		klog.Infof("Resync triggered for service: %s", serviceKey)
	}

	// Create stop channel
	stopCh := make(chan struct{})
	defer close(stopCh)

	// Create the endpoint slice watcher
	_ = NewEndpointSliceWatcher(client, stopCh, resyncCallback)

	// Create a service with externalTrafficPolicy=Cluster initially
	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-service",
			Namespace: "default",
		},
		Spec: corev1.ServiceSpec{
			Type:                  corev1.ServiceTypeLoadBalancer,
			ExternalTrafficPolicy: corev1.ServiceExternalTrafficPolicyTypeCluster,
		},
	}

	_, err := client.CoreV1().Services("default").Create(context.Background(), service, metav1.CreateOptions{})
	require.NoError(t, err)

	// Wait a bit for the informer to process the service
	time.Sleep(100 * time.Millisecond)

	// Update service to change externalTrafficPolicy to Local
	updatedService := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-service",
			Namespace: "default",
		},
		Spec: corev1.ServiceSpec{
			Type:                  corev1.ServiceTypeLoadBalancer,
			ExternalTrafficPolicy: corev1.ServiceExternalTrafficPolicyTypeLocal,
		},
	}

	_, err = client.CoreV1().Services("default").Update(context.Background(), updatedService, metav1.UpdateOptions{})
	require.NoError(t, err)

	// Wait for the resync to be triggered
	time.Sleep(200 * time.Millisecond)

	// Check that resync was triggered
	resyncMutex.Lock()
	defer resyncMutex.Unlock()

	assert.Contains(t, resyncCalls, "default/test-service", "Expected resync to be triggered when externalTrafficPolicy changed to Local")
}

func TestEndpointSliceWatcher_HasNodeAssignmentChanged(t *testing.T) {
	watcher := &EndpointSliceWatcher{}

	// Test case 1: Same nodes, no change
	oldEpSlice := &discoveryv1.EndpointSlice{
		Endpoints: []discoveryv1.Endpoint{
			{
				NodeName: ptr.To("node-1"),
				Conditions: discoveryv1.EndpointConditions{
					Ready: ptr.To(true),
				},
			},
			{
				NodeName: ptr.To("node-2"),
				Conditions: discoveryv1.EndpointConditions{
					Ready: ptr.To(true),
				},
			},
		},
	}

	newEpSlice := &discoveryv1.EndpointSlice{
		Endpoints: []discoveryv1.Endpoint{
			{
				NodeName: ptr.To("node-1"),
				Conditions: discoveryv1.EndpointConditions{
					Ready: ptr.To(true),
				},
			},
			{
				NodeName: ptr.To("node-2"),
				Conditions: discoveryv1.EndpointConditions{
					Ready: ptr.To(true),
				},
			},
		},
	}

	assert.False(t, watcher.hasNodeAssignmentChanged(oldEpSlice, newEpSlice), "Should not detect change when nodes are the same")

	// Test case 2: Different nodes, should detect change
	newEpSlice2 := &discoveryv1.EndpointSlice{
		Endpoints: []discoveryv1.Endpoint{
			{
				NodeName: ptr.To("node-1"),
				Conditions: discoveryv1.EndpointConditions{
					Ready: ptr.To(true),
				},
			},
			{
				NodeName: ptr.To("node-3"), // Changed from node-2 to node-3
				Conditions: discoveryv1.EndpointConditions{
					Ready: ptr.To(true),
				},
			},
		},
	}

	assert.True(t, watcher.hasNodeAssignmentChanged(oldEpSlice, newEpSlice2), "Should detect change when nodes are different")

	// Test case 3: Different number of nodes
	newEpSlice3 := &discoveryv1.EndpointSlice{
		Endpoints: []discoveryv1.Endpoint{
			{
				NodeName: ptr.To("node-1"),
				Conditions: discoveryv1.EndpointConditions{
					Ready: ptr.To(true),
				},
			},
			// Missing node-2
		},
	}

	assert.True(t, watcher.hasNodeAssignmentChanged(oldEpSlice, newEpSlice3), "Should detect change when number of nodes is different")
}
