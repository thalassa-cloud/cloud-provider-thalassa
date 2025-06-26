package provider

import (
	"fmt"
	"sync"

	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	discoverylisters "k8s.io/client-go/listers/discovery/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
)

type EndpointSliceWatcher struct {
	informer        cache.SharedIndexInformer
	epSliceInformer informers.SharedInformerFactory
	serviceInformer informers.SharedInformerFactory

	// Callback function to trigger load balancer resync
	onEndpointSliceChange func(serviceKey string)

	// Track services that have externalTrafficPolicy=Local
	localTrafficServices sync.Map

	mu sync.RWMutex
}

func NewEndpointSliceWatcher(
	client kubernetes.Interface,
	stopCh <-chan struct{},
	onEndpointSliceChange func(serviceKey string),
) *EndpointSliceWatcher {
	w := &EndpointSliceWatcher{
		onEndpointSliceChange: onEndpointSliceChange,
	}

	// Create informer factory for endpoint slices
	epSliceFactory := informers.NewSharedInformerFactory(client, 0)
	w.epSliceInformer = epSliceFactory
	w.informer = w.epSliceInformer.Discovery().V1().EndpointSlices().Informer()

	// Create informer factory for services to track externalTrafficPolicy
	serviceFactory := informers.NewSharedInformerFactory(client, 0)
	w.serviceInformer = serviceFactory

	// Add event handlers for endpoint slices
	w.informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    w.handleEndpointSliceAdd,
		UpdateFunc: w.handleEndpointSliceUpdate,
		DeleteFunc: w.handleEndpointSliceDelete,
	})

	// Add event handlers for services to track externalTrafficPolicy changes
	serviceInformer := w.serviceInformer.Core().V1().Services().Informer()
	serviceInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    w.handleServiceAdd,
		UpdateFunc: w.handleServiceUpdate,
		DeleteFunc: w.handleServiceDelete,
	})

	// Start informers
	epSliceFactory.Start(stopCh)
	serviceFactory.Start(stopCh)

	// Wait for caches to sync
	cache.WaitForCacheSync(stopCh, w.informer.HasSynced, serviceInformer.HasSynced)

	return w
}

// handleEndpointSliceAdd handles endpoint slice creation events
func (w *EndpointSliceWatcher) handleEndpointSliceAdd(obj interface{}) {
	epSlice, ok := obj.(*discoveryv1.EndpointSlice)
	if !ok {
		klog.Errorf("Expected EndpointSlice but got %T", obj)
		return
	}

	serviceKey := w.getServiceKeyFromEndpointSlice(epSlice)
	if serviceKey == "" {
		return
	}

	// Only trigger resync if this service has externalTrafficPolicy=Local
	if w.hasLocalTrafficPolicy(serviceKey) {
		klog.Infof("Endpoint slice added for service %s, triggering resync", serviceKey)
		w.onEndpointSliceChange(serviceKey)
	}
}

// handleEndpointSliceUpdate handles endpoint slice update events
func (w *EndpointSliceWatcher) handleEndpointSliceUpdate(oldObj, newObj interface{}) {
	oldEpSlice, ok := oldObj.(*discoveryv1.EndpointSlice)
	if !ok {
		klog.Errorf("Expected EndpointSlice but got %T", oldObj)
		return
	}
	klog.Infof("Endpoint slice updated: %s", oldEpSlice.Name)

	newEpSlice, ok := newObj.(*discoveryv1.EndpointSlice)
	if !ok {
		klog.Errorf("Expected EndpointSlice but got %T", newObj)
		return
	}

	serviceKey := w.getServiceKeyFromEndpointSlice(newEpSlice)
	if serviceKey == "" {
		return
	}

	// Only trigger resync if this service has externalTrafficPolicy=Local
	if !w.hasLocalTrafficPolicy(serviceKey) {
		klog.Infof("Endpoint slice updated for service %s, but it does not have externalTrafficPolicy=Local, skipping resync", serviceKey)
		return
	}
	klog.Infof("Endpoint slice updated for service %s, triggering resync", serviceKey)

	// Check if node assignments have changed
	if w.hasNodeAssignmentChanged(oldEpSlice, newEpSlice) {
		klog.V(4).Infof("Node assignments changed for service %s, triggering resync", serviceKey)
		w.onEndpointSliceChange(serviceKey)
	}
}

// handleEndpointSliceDelete handles endpoint slice deletion events
func (w *EndpointSliceWatcher) handleEndpointSliceDelete(obj interface{}) {
	epSlice, ok := obj.(*discoveryv1.EndpointSlice)
	if !ok {
		klog.Errorf("Expected EndpointSlice but got %T", obj)
		return
	}

	serviceKey := w.getServiceKeyFromEndpointSlice(epSlice)
	if serviceKey == "" {
		return
	}

	// Only trigger resync if this service has externalTrafficPolicy=Local
	if w.hasLocalTrafficPolicy(serviceKey) {
		klog.V(4).Infof("Endpoint slice deleted for service %s, triggering resync", serviceKey)
		w.onEndpointSliceChange(serviceKey)
	}
}

// handleServiceAdd handles service creation events
func (w *EndpointSliceWatcher) handleServiceAdd(obj interface{}) {
	svc, ok := obj.(*corev1.Service)
	if !ok {
		klog.Errorf("Expected Service but got %T", obj)
		return
	}

	serviceKey := fmt.Sprintf("%s/%s", svc.Namespace, svc.Name)

	if svc.Spec.ExternalTrafficPolicy == corev1.ServiceExternalTrafficPolicyTypeLocal {
		w.localTrafficServices.Store(serviceKey, struct{}{})
		klog.V(4).Infof("Service %s added with externalTrafficPolicy=Local", serviceKey)
	}
}

// handleServiceUpdate handles service update events
func (w *EndpointSliceWatcher) handleServiceUpdate(oldObj, newObj interface{}) {
	oldSvc, ok := oldObj.(*corev1.Service)
	if !ok {
		klog.Errorf("Expected Service but got %T", oldObj)
		return
	}

	newSvc, ok := newObj.(*corev1.Service)
	if !ok {
		klog.Errorf("Expected Service but got %T", newObj)
		return
	}

	serviceKey := fmt.Sprintf("%s/%s", newSvc.Namespace, newSvc.Name)

	oldPolicy := oldSvc.Spec.ExternalTrafficPolicy
	newPolicy := newSvc.Spec.ExternalTrafficPolicy

	// If externalTrafficPolicy changed to Local, add to tracking
	if oldPolicy != corev1.ServiceExternalTrafficPolicyTypeLocal && newPolicy == corev1.ServiceExternalTrafficPolicyTypeLocal {
		w.localTrafficServices.Store(serviceKey, struct{}{})
		klog.V(4).Infof("Service %s changed to externalTrafficPolicy=Local, triggering resync", serviceKey)
		w.onEndpointSliceChange(serviceKey)
	}

	// If externalTrafficPolicy changed from Local, remove from tracking
	if oldPolicy == corev1.ServiceExternalTrafficPolicyTypeLocal && newPolicy != corev1.ServiceExternalTrafficPolicyTypeLocal {
		w.localTrafficServices.Delete(serviceKey)
		klog.V(4).Infof("Service %s changed from externalTrafficPolicy=Local, triggering resync", serviceKey)
		w.onEndpointSliceChange(serviceKey)
	}
}

// handleServiceDelete handles service deletion events
func (w *EndpointSliceWatcher) handleServiceDelete(obj interface{}) {
	svc, ok := obj.(*corev1.Service)
	if !ok {
		klog.Errorf("Expected Service but got %T", obj)
		return
	}

	serviceKey := fmt.Sprintf("%s/%s", svc.Namespace, svc.Name)
	w.localTrafficServices.Delete(serviceKey)
	klog.V(4).Infof("Service %s deleted, removed from local traffic tracking", serviceKey)
}

// getServiceKeyFromEndpointSlice extracts the service key from an endpoint slice
func (w *EndpointSliceWatcher) getServiceKeyFromEndpointSlice(epSlice *discoveryv1.EndpointSlice) string {
	serviceName, ok := epSlice.Labels[discoveryv1.LabelServiceName]
	if !ok {
		return ""
	}
	return fmt.Sprintf("%s/%s", epSlice.Namespace, serviceName)
}

// hasLocalTrafficPolicy checks if a service has externalTrafficPolicy=Local
func (w *EndpointSliceWatcher) hasLocalTrafficPolicy(serviceKey string) bool {
	_, exists := w.localTrafficServices.Load(serviceKey)
	return exists
}

// hasNodeAssignmentChanged checks if node assignments have changed between two endpoint slices
func (w *EndpointSliceWatcher) hasNodeAssignmentChanged(oldEpSlice, newEpSlice *discoveryv1.EndpointSlice) bool {
	oldNodes := make(map[string]struct{})
	newNodes := make(map[string]struct{})

	// Collect old nodes
	for _, ep := range oldEpSlice.Endpoints {
		if ep.NodeName != nil && ep.Conditions.Ready != nil && *ep.Conditions.Ready {
			oldNodes[*ep.NodeName] = struct{}{}
		}
	}

	// Collect new nodes
	for _, ep := range newEpSlice.Endpoints {
		if ep.NodeName != nil && ep.Conditions.Ready != nil && *ep.Conditions.Ready {
			newNodes[*ep.NodeName] = struct{}{}
		}
	}

	// Check if the sets are different
	if len(oldNodes) != len(newNodes) {
		return true
	}

	for nodeName := range oldNodes {
		if _, exists := newNodes[nodeName]; !exists {
			return true
		}
	}

	return false
}

// GetEndpointSliceLister returns the endpoint slice lister
func (w *EndpointSliceWatcher) GetEndpointSliceLister() discoverylisters.EndpointSliceLister {
	return w.epSliceInformer.Discovery().V1().EndpointSlices().Lister()
}
