package provider

import (
	"context"
	"fmt"
	"net"
	"strings"

	"github.com/thalassa-cloud/client-go/iaas"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/klog/v2"
)

func (lb *loadbalancer) getTargetGroupIdentityForListener(service *corev1.Service, listener iaas.VpcLoadbalancerListener, targetGroups []iaas.VpcLoadbalancerTargetGroup) string {
	klog.Infof("getting target group identity for listener %q", listener.Name)
	desiredLabels := lb.GetLabelsForVpcLoadbalancerTargetGroup(service, listener.Port, string(listener.Protocol))
	for _, targetGroup := range targetGroups {
		klog.Infof("checking target group %q", targetGroup.Identity)
		// match op basis van port en protocol labels
		if !matchLabels(desiredLabels, targetGroup.Labels) {
			klog.Infof("target group %q does not match desired labels, skipping: Labels: %v, Desired: %v", targetGroup.Identity, targetGroup.Labels, desiredLabels)
			continue
		}
		klog.Infof("found target group %q for listener %q", targetGroup.Identity, listener.Port)
		return targetGroup.Identity
	}
	klog.Infof("no target group identity found for listener %q", listener.Name)
	return ""
}

func (lb *loadbalancer) updateVpcLoadbalancerListener(ctx context.Context, service *corev1.Service, loadbalancer *iaas.VpcLoadbalancer, desiredListeners []iaas.VpcLoadbalancerListener, targetGroups []iaas.VpcLoadbalancerTargetGroup) error {
	existingListenersForLoadBalancer, err := lb.iaasClient.ListListeners(ctx, &iaas.ListLoadbalancerListenersRequest{
		Loadbalancer: loadbalancer.Identity,
	})
	if err != nil {
		return fmt.Errorf("failed to list listeners: %v", err)
	}

	desiredListenersPortMap := map[int]iaas.VpcLoadbalancerListener{}
	for _, listener := range desiredListeners {
		desiredListenersPortMap[listener.Port] = listener
	}

	existingListenersPortMap := map[int]iaas.VpcLoadbalancerListener{}
	for _, listener := range existingListenersForLoadBalancer {
		existingListenersPortMap[listener.Port] = listener
	}

	if !equality.Semantic.DeepEqual(desiredListeners, existingListenersForLoadBalancer) {
		// check which listeners to delete
		for _, listener := range existingListenersForLoadBalancer {
			if listenerToUpdate, ok := desiredListenersPortMap[listener.Port]; !ok {
				klog.Infof("deleting listener %q for loadbalancer %q", listener.Name, loadbalancer.Name)
				if err := lb.iaasClient.DeleteListener(ctx, loadbalancer.Identity, listener.Identity); err != nil {
					return fmt.Errorf("failed to delete listener: %v", err)
				}
			} else {
				// TODO: only update the listener if the desired listener is different from the existing listener
				// make sure the listener is up-to-date

				targetGroupIdentity := lb.getTargetGroupIdentityForListener(service, listenerToUpdate, targetGroups)
				if targetGroupIdentity == "" {
					klog.Infof("WARNING: existing listener %q - target group identity is empty, skipping", listenerToUpdate.Name)
					continue
				}
				klog.Infof("updating listener %q for loadbalancer %q with target group %q", listenerToUpdate.Name, loadbalancer.Name, targetGroupIdentity)
				// update the listener
				if _, err := lb.iaasClient.UpdateListener(ctx, loadbalancer.Identity, listener.Identity, iaas.UpdateListener{
					Name:        listenerToUpdate.Name,
					Description: listenerToUpdate.Description,
					Labels:      listenerToUpdate.Labels,
					Annotations: listenerToUpdate.Annotations,

					// routing info
					Port:        listenerToUpdate.Port,
					Protocol:    listenerToUpdate.Protocol,
					TargetGroup: targetGroupIdentity,

					// TODO: support ACLs
					// AllowedSources: listenerToUpdate.AllowedSources,
				}); err != nil {
					return fmt.Errorf("failed to update listener: %v", err)
				}
			}
		}
	}

	// create missing listeners
	for _, listener := range desiredListeners {
		if _, ok := existingListenersPortMap[listener.Port]; !ok {
			targetGroupIdentity := lb.getTargetGroupIdentityForListener(service, listener, targetGroups)
			if targetGroupIdentity == "" {
				klog.Infof("WARNING: desired listener %q - target group identity is empty, skipping", listener.Name)
				continue
			}
			klog.Infof("creating listener %q for loadbalancer %q with target group %q", listener.Name, loadbalancer.Name, targetGroupIdentity)
			if _, err := lb.iaasClient.CreateListener(ctx, loadbalancer.Identity, iaas.CreateListener{
				Name:        listener.Name,
				Description: listener.Description,
				Labels:      listener.Labels,
				Annotations: listener.Annotations,
				// routing info
				Port:           listener.Port,
				Protocol:       listener.Protocol,
				TargetGroup:    targetGroupIdentity,
				AllowedSources: listener.AllowedSources,
			}); err != nil {
				return fmt.Errorf("failed to create listener: %v", err)
			}
		}
	}
	return nil
}

func (lb *loadbalancer) desiredVpcLoadbalancerListener(service *corev1.Service) []iaas.VpcLoadbalancerListener {
	aclAllowedSources := []string{}
	if val, ok := service.Annotations[LoadbalancerAnnotationAclAllowedSources]; ok {
		sources := strings.Split(val, ",")
		// validate that each entry is an IP or CIDR
		for _, source := range sources {
			if _, _, err := net.ParseCIDR(source); err != nil {
				klog.Errorf("invalid CIDR in acl-allowed-sources annotation: %v", err)
				continue
			}
			aclAllowedSources = append(aclAllowedSources, source)
		}
	}

	listener := make([]iaas.VpcLoadbalancerListener, len(service.Spec.Ports))
	for i, port := range service.Spec.Ports {
		listener[i].Name = getPortName(lb.GetLoadBalancerName(context.Background(), lb.cluster, service), port)
		listener[i].Description = fmt.Sprintf("Listener for Kubernetes service %s", service.GetName())
		listener[i].Protocol = iaas.LoadbalancerProtocol(strings.ToLower(string(port.Protocol)))
		listener[i].Port = int(port.Port)
		listener[i].TargetGroup = &iaas.VpcLoadbalancerTargetGroup{
			// TODO: determine the target group name and identity
			Name: port.Name,
		}
		listener[i].Labels = lb.GetLabelsForVpcLoadbalancerTargetGroup(service, int(port.Port), string(port.Protocol))
		listener[i].Annotations = lb.GetAnnotationsForVpcLoadbalancer(service)
		listener[i].AllowedSources = aclAllowedSources
	}
	return listener
}
