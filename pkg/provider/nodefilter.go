package provider

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	"k8s.io/apimachinery/pkg/labels"
	discoverylisters "k8s.io/client-go/listers/discovery/v1"
	"k8s.io/klog/v2"
)

type NodeFilter struct {
	epSliceLister discoverylisters.EndpointSliceLister
}

// Filter drops every node that does NOT host a ready endpoint for the Service
// when externalTrafficPolicy is Local. For Cluster policy we leave the list intact.
func (f *NodeFilter) Filter(
	ctx context.Context,
	svc *corev1.Service,
	nodes []*corev1.Node,
) ([]*corev1.Node, error) {

	if svc.Spec.ExternalTrafficPolicy != corev1.ServiceExternalTrafficPolicyTypeLocal {
		return nodes, nil
	}
	klog.Infof("Filtering nodes for service %s in namespace %s", svc.Name, svc.Namespace)

	readyNodes := map[string]struct{}{}
	slices, err := f.epSliceLister.EndpointSlices(svc.Namespace).List(labels.Set{discoveryv1.LabelServiceName: svc.Name}.AsSelector())
	if err != nil {
		return nil, err
	}

	if len(slices) == 0 {
		klog.Infof("No endpoint slices found for service %s in namespace %s", svc.Name, svc.Namespace)
		return nodes, nil
	}

	for _, sl := range slices {
		for _, ep := range sl.Endpoints {
			if ep.NodeName == nil {
				continue
			}
			if ep.Conditions.Ready != nil && !*ep.Conditions.Ready {
				continue
			}
			if ep.Conditions.Terminating != nil && *ep.Conditions.Terminating {
				continue
			}
			readyNodes[*ep.NodeName] = struct{}{}
		}
	}

	if len(readyNodes) == 0 {
		klog.Infof("No ready nodes found for service %s in namespace %s", svc.Name, svc.Namespace)
		return nodes, nil
	}

	var filtered []*corev1.Node
	for _, n := range nodes {
		if _, ok := readyNodes[n.Name]; ok {
			filtered = append(filtered, n)
		} else {
			klog.Infof("Node %s is not available for service %s in namespace %s", n.Name, svc.Name, svc.Namespace)
		}
	}
	klog.Infof("Filtered %d nodes for service %s in namespace %s", len(filtered), svc.Name, svc.Namespace)
	return filtered, nil
}
