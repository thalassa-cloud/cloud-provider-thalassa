package provider

import (
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

type EndpointSliceWatcher struct {
	informer        cache.SharedIndexInformer
	epSliceInformer informers.SharedInformerFactory
}

func NewEndpointSliceWatcher(
	client kubernetes.Interface,
	stopCh <-chan struct{},
) *EndpointSliceWatcher {
	w := &EndpointSliceWatcher{}
	factory := informers.NewSharedInformerFactory(client, 0) // 0 = no resync
	w.epSliceInformer = factory
	w.informer = w.epSliceInformer.Discovery().V1().EndpointSlices().Informer()
	factory.Start(stopCh)
	cache.WaitForCacheSync(stopCh, w.informer.HasSynced)

	return w
}
