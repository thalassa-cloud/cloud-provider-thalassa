package provider

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/client-go/kubernetes/fake"
)

func TestCloud_LoadBalancerIsSingleton(t *testing.T) {
	stopCh := make(chan struct{})

	cloud := &Cloud{
		config: CloudConfig{
			LoadBalancer: LoadBalancerConfig{Enabled: true},
			VpcIdentity:  "vpc-1",
			Cluster:      "test-cluster",
		},
		endpointSlicesClient: fake.NewSimpleClientset(),
		stopCh:               stopCh,
	}

	first, ok := cloud.LoadBalancer()
	require.True(t, ok)
	require.NotNil(t, first)

	second, ok := cloud.LoadBalancer()
	require.True(t, ok)
	assert.Same(t, first, second)
}

func TestCloud_LoadBalancerCleanupOnStop(t *testing.T) {
	stopCh := make(chan struct{})

	cloud := &Cloud{
		config: CloudConfig{
			LoadBalancer: LoadBalancerConfig{Enabled: true},
			VpcIdentity:  "vpc-1",
			Cluster:      "test-cluster",
		},
		endpointSlicesClient: fake.NewSimpleClientset(),
		stopCh:               stopCh,
	}

	lbIface, ok := cloud.LoadBalancer()
	require.True(t, ok)

	lb, ok := lbIface.(*loadbalancer)
	require.True(t, ok)

	close(stopCh)

	require.Eventually(t, func() bool {
		select {
		case <-lb.ctx.Done():
			return true
		default:
			return false
		}
	}, time.Second, 10*time.Millisecond)
}
