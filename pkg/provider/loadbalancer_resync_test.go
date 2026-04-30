package provider

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/util/workqueue"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadBalancer_EnqueueLocalTrafficPolicyLoadBalancers(t *testing.T) {
	client := fake.NewSimpleClientset(
		&corev1.Service{
			ObjectMeta: metav1.ObjectMeta{Name: "lb-local", Namespace: "default"},
			Spec: corev1.ServiceSpec{
				Type:                  corev1.ServiceTypeLoadBalancer,
				ExternalTrafficPolicy: corev1.ServiceExternalTrafficPolicyTypeLocal,
			},
		},
		&corev1.Service{
			ObjectMeta: metav1.ObjectMeta{Name: "lb-cluster", Namespace: "default"},
			Spec: corev1.ServiceSpec{
				Type:                  corev1.ServiceTypeLoadBalancer,
				ExternalTrafficPolicy: corev1.ServiceExternalTrafficPolicyTypeCluster,
			},
		},
		&corev1.Service{
			ObjectMeta: metav1.ObjectMeta{Name: "clusterip-local", Namespace: "default"},
			Spec: corev1.ServiceSpec{
				Type:                  corev1.ServiceTypeClusterIP,
				ExternalTrafficPolicy: corev1.ServiceExternalTrafficPolicyTypeLocal,
			},
		},
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	lb := &loadbalancer{
		endpointSlicesClient: client,
		serviceQueue: workqueue.NewTypedRateLimitingQueueWithConfig(
			workqueue.NewTypedItemExponentialFailureRateLimiter[string](0, 0),
			workqueue.TypedRateLimitingQueueConfig[string]{Name: "test-queue"},
		),
		ctx: ctx,
	}

	lb.enqueueLocalTrafficPolicyLoadBalancers()

	require.Equal(t, 1, lb.serviceQueue.Len())
	item, shutdown := lb.serviceQueue.Get()
	require.False(t, shutdown)
	assert.Equal(t, "default/lb-local", item)
	lb.serviceQueue.Done(item)
}

