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
	"k8s.io/utils/ptr"
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

	desiredListenersPortProtocolMap := map[string]iaas.VpcLoadbalancerListener{}
	for _, listener := range desiredListeners {
		desiredListenersPortProtocolMap[listenerPortProtocolKey(listener.Protocol, listener.Port)] = listener
	}

	existingListenersPortProtocolMap := map[string]iaas.VpcLoadbalancerListener{}
	for _, listener := range existingListenersForLoadBalancer {
		existingListenersPortProtocolMap[listenerPortProtocolKey(listener.Protocol, listener.Port)] = listener
	}

	if !equality.Semantic.DeepEqual(desiredListeners, existingListenersForLoadBalancer) {
		// check which listeners to delete
		for _, listener := range existingListenersForLoadBalancer {
			if listenerToUpdate, ok := desiredListenersPortProtocolMap[listenerPortProtocolKey(listener.Protocol, listener.Port)]; !ok {
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
					Name:                  listenerToUpdate.Name,
					Description:           listenerToUpdate.Description,
					Labels:                listenerToUpdate.Labels,
					Annotations:           listenerToUpdate.Annotations,
					Port:                  listenerToUpdate.Port,
					Protocol:              listenerToUpdate.Protocol,
					TargetGroup:           targetGroupIdentity,
					ConnectionIdleTimeout: listenerToUpdate.ConnectionIdleTimeout,
					MaxConnections:        listenerToUpdate.MaxConnections,
					AllowedSources:        listenerToUpdate.AllowedSources,
				}); err != nil {
					return fmt.Errorf("failed to update listener: %v", err)
				}
			}
		}
	}

	// create missing listeners
	for _, listener := range desiredListeners {
		if _, ok := existingListenersPortProtocolMap[listenerPortProtocolKey(listener.Protocol, listener.Port)]; !ok {
			targetGroupIdentity := lb.getTargetGroupIdentityForListener(service, listener, targetGroups)
			if targetGroupIdentity == "" {
				klog.Infof("WARNING: desired listener %q - target group identity is empty, skipping", listener.Name)
				continue
			}
			klog.Infof("creating listener %q for loadbalancer %q with target group %q", listener.Name, loadbalancer.Name, targetGroupIdentity)
			if _, err := lb.iaasClient.CreateListener(ctx, loadbalancer.Identity, iaas.CreateListener{
				Name:                  listener.Name,
				Description:           listener.Description,
				Labels:                listener.Labels,
				Annotations:           listener.Annotations,
				Port:                  listener.Port,
				Protocol:              listener.Protocol,
				TargetGroup:           targetGroupIdentity,
				AllowedSources:        listener.AllowedSources,
				ConnectionIdleTimeout: listener.ConnectionIdleTimeout,
				MaxConnections:        listener.MaxConnections,
			}); err != nil {
				return fmt.Errorf("failed to create listener: %v", err)
			}
		}
	}
	return nil
}

func (lb *loadbalancer) desiredVpcLoadbalancerListener(service *corev1.Service) []iaas.VpcLoadbalancerListener {
	globalAclAllowedSources := lb.getGlobalAclAllowedSources(service)

	connectionTimeout, err := getIntAnnotation(service, LoadbalancerAnnotationIdleConnectionTimeout, DefaultIdleConnectionTimeout)
	if err != nil {
		klog.Errorf("failed to get idle connection timeout: %v", err)
	}
	maxConnections, err := getIntAnnotation(service, LoadbalancerAnnotationMaxConnections, DefaultMaxConnections)
	if err != nil {
		klog.Errorf("failed to get max connections: %v", err)
	}

	listener := make([]iaas.VpcLoadbalancerListener, len(service.Spec.Ports))
	for i, port := range service.Spec.Ports {
		// Get per-port ACL allowed sources
		perPortAclAllowedSources := lb.getPerPortAclAllowedSources(service, port)

		// Combine global and per-port ACL sources (union)
		combinedAclAllowedSources := lb.removeDuplicateStrings(append(globalAclAllowedSources, perPortAclAllowedSources...))

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
		listener[i].AllowedSources = combinedAclAllowedSources
		listener[i].ConnectionIdleTimeout = ptr.To(uint32(connectionTimeout))
		listener[i].MaxConnections = ptr.To(uint32(maxConnections))
	}
	return listener
}

// getGlobalAclAllowedSources returns global allowed sources from Service.spec.loadBalancerSourceRanges
// and the Thalassa acl-allowed-sources annotation. Sources from both are combined (union).
func (lb *loadbalancer) getGlobalAclAllowedSources(service *corev1.Service) []string {
	var sources []string

	sources = append(sources, lb.validateAclSources(service.Spec.LoadBalancerSourceRanges, "service.spec.loadBalancerSourceRanges")...)

	if val, ok := service.Annotations[LoadbalancerAnnotationAclAllowedSources]; ok {
		sources = append(sources, lb.parseAclSources(val)...)
	}

	result := lb.removeDuplicateStrings(sources)
	if result == nil {
		return []string{}
	}
	return result
}

// getPerPortAclAllowedSources returns the allowed sources for a specific port by checking both port name and port number annotations
func (lb *loadbalancer) getPerPortAclAllowedSources(service *corev1.Service, port corev1.ServicePort) []string {
	var allowedSources []string

	// Check for port name annotation first (e.g., loadbalancer.k8s.thalassa.cloud/acl-port-http)
	if port.Name != "" {
		portNameAnnotation := fmt.Sprintf("%s-%s", LoadbalancerAnnotationAclAllowedSourcesPort, port.Name)
		if val, ok := service.Annotations[portNameAnnotation]; ok {
			sources := lb.parseAclSources(val)
			allowedSources = append(allowedSources, sources...)
		}
	}

	// Check for port number annotation (e.g., loadbalancer.k8s.thalassa.cloud/acl-port-80)
	portNumberAnnotation := fmt.Sprintf("%s-%d", LoadbalancerAnnotationAclAllowedSourcesPort, port.Port)
	if val, ok := service.Annotations[portNumberAnnotation]; ok {
		sources := lb.parseAclSources(val)
		allowedSources = append(allowedSources, sources...)
	}

	// Remove duplicates while preserving order
	result := lb.removeDuplicateStrings(allowedSources)
	if result == nil {
		return []string{}
	}
	return result
}

// parseAclSources parses a comma-separated string of CIDR ranges and validates each one
func (lb *loadbalancer) parseAclSources(sourcesStr string) []string {
	return lb.validateAclSources(strings.Split(sourcesStr, ","), "acl-allowed-sources annotation")
}

// validateAclSources validates a list of CIDR ranges, skipping empty and invalid entries
func (lb *loadbalancer) validateAclSources(sources []string, sourceName string) []string {
	validSources := make([]string, 0, len(sources))

	for _, source := range sources {
		source = strings.TrimSpace(source)
		if source == "" {
			continue
		}

		if _, _, err := net.ParseCIDR(source); err != nil {
			klog.Errorf("invalid CIDR in %s: %v", sourceName, err)
			continue
		}
		validSources = append(validSources, source)
	}

	return validSources
}

func listenerPortProtocolKey(protocol iaas.LoadbalancerProtocol, port int) string {
	return fmt.Sprintf("%s:%d", protocol, port)
}

// removeDuplicateStrings removes duplicate strings from a slice while preserving order
func (lb *loadbalancer) removeDuplicateStrings(input []string) []string {
	keys := make(map[string]bool)
	var result []string

	for _, item := range input {
		if !keys[item] {
			keys[item] = true
			result = append(result, item)
		}
	}

	return result
}
