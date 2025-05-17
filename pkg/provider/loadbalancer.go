package provider

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/thalassa-cloud/client-go/filters"
	"github.com/thalassa-cloud/client-go/iaas"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	cloudprovider "k8s.io/cloud-provider"
	"k8s.io/klog/v2"
	"k8s.io/utils/ptr"
)

type AvailabilityZones []string

const (
	// Default interval between polling the service after creation
	defaultLoadBalancerCreatePollInterval = 5 * time.Second

	// Default timeout between polling the service after creation
	defaultLoadBalancerCreatePollTimeout = 5 * time.Minute
)

// loadbalancer represents a load balancer configuration and its associated resources.
// It includes the namespace, client, configuration, and infrastructure labels.
// Additionally, it holds information about the tenant VPC name and external network details.
type loadbalancer struct {
	iaasClient *iaas.Client

	config           LoadBalancerConfig
	additionalLabels map[string]string

	vpcIdentity   string
	defaultSubnet string
	cluster       string
}

// GetLoadBalancer returns the load balancerstatus for the specified service.
func (lb *loadbalancer) GetLoadBalancer(ctx context.Context, clusterName string, service *corev1.Service) (*corev1.LoadBalancerStatus, bool, error) {
	vpcLoadbalancer, err := lb.fetchVpcLoadbalancerFromCloud(ctx, clusterName, service)
	if err != nil {
		klog.Errorf("failed to get LoadBalancer for service: %v", err)
		return nil, false, err
	}
	if vpcLoadbalancer == nil {
		return nil, false, nil
	}

	loadbalancerStatus := &corev1.LoadBalancerStatus{
		Ingress: []corev1.LoadBalancerIngress{},
	}

	for _, ip := range vpcLoadbalancer.ExternalIpAddresses {
		if ip != "" {
			loadbalancerStatus.Ingress = append(loadbalancerStatus.Ingress, corev1.LoadBalancerIngress{
				IP:       ip,
				Hostname: vpcLoadbalancer.Hostname,
				IPMode:   ptr.To(corev1.LoadBalancerIPModeProxy),
			})
		}
	}
	return loadbalancerStatus, true, nil
}

// GetLoadBalancerName returns the name of the load balancer for the specified service.
func (lb *loadbalancer) GetLoadBalancerName(ctx context.Context, clusterName string, service *corev1.Service) string {
	return cloudprovider.DefaultLoadBalancerName(service)
}

// EnsureLoadBalancer creates a new load balancer 'name', or updates the existing one. Returns the status of the balancer
// Implementations must treat the *v1.Service and *v1.Node
// parameters as read-only and not modify them.
// Parameter 'clusterName' is the name of the cluster as presented to kube-controller-manager
func (lb *loadbalancer) EnsureLoadBalancer(ctx context.Context, clusterName string, service *corev1.Service, nodes []*corev1.Node) (*corev1.LoadBalancerStatus, error) {
	klog.Infof("EnsureLoadBalancer for service %s", service.GetName())

	vpcLoadbalancer, err := lb.fetchVpcLoadbalancerFromCloud(ctx, clusterName, service)
	if err != nil {
		klog.Errorf("Failed to get LoadBalancer service: %v", err)
		return nil, err
	}

	if vpcLoadbalancer != nil {
		klog.Infof("LoadBalancer service %s already exists, updating existing listener and target groups", vpcLoadbalancer.Identity)
		return lb.updateVpcLoadbalancerListenersAndTargetGroups(ctx, clusterName, service, nodes, vpcLoadbalancer)
	}

	klog.Infof("LoadBalancer service %s does not exist, creating new one", service.GetName())

	lbName := lb.GetLoadBalancerName(ctx, clusterName, service)
	vpcLoadbalancer, err = lb.createVpcLoadbalancer(ctx, lbName, service)
	if err != nil {
		klog.Errorf("failed to create LoadBalancer service: %v", err)
		return nil, err
	}
	klog.Infof("LoadBalancer %q for service %q created, updating listener and target groups", vpcLoadbalancer.Identity, service.GetName())

	if status, err := lb.updateVpcLoadbalancerListenersAndTargetGroups(ctx, clusterName, service, nodes, vpcLoadbalancer); err != nil {
		return status, err
	}
	klog.Infof("LoadBalancer %q for service %q updated, waiting for loadbalancer to be ready", vpcLoadbalancer.Identity, service.GetName())

	// now we wait for the loadbalancer to be ready
	err = wait.PollUntilContextTimeout(ctx, lb.getLoadBalancerCreatePollInterval(), lb.getLoadBalancerCreatePollTimeout(), true, func(ctx context.Context) (bool, error) {
		if vpcLoadbalancer.Status == "ready" && len(vpcLoadbalancer.ExternalIpAddresses) > 0 {
			return true, nil
		}
		var vpcLB *iaas.VpcLoadbalancer
		vpcLB, err = lb.fetchVpcLoadbalancerFromCloud(ctx, clusterName, service)
		if err != nil {
			klog.Errorf("Failed to get LoadBalancer service: %v", err)
			return false, err
		}
		if vpcLB.Status == "ready" && len(vpcLB.ExternalIpAddresses) > 0 {
			vpcLoadbalancer = vpcLB
			return true, nil
		}
		return false, nil
	})

	if err != nil {
		klog.Errorf("failed to poll VPC LoadBalancer service: %v", err)
		return nil, err
	}

	klog.Infof("LoadBalancer %q for service %q is ready", vpcLoadbalancer.Identity, service.GetName())

	loadbalancerStatus := &corev1.LoadBalancerStatus{
		Ingress: []corev1.LoadBalancerIngress{},
	}
	for _, ip := range vpcLoadbalancer.ExternalIpAddresses {
		if ip != "" {
			loadbalancerStatus.Ingress = append(loadbalancerStatus.Ingress, corev1.LoadBalancerIngress{
				IP:       ip,
				Hostname: vpcLoadbalancer.Hostname,
				IPMode:   ptr.To(corev1.LoadBalancerIPModeProxy),
			})
		}
	}
	return loadbalancerStatus, nil
}

// UpdateLoadBalancer updates the ports in the LoadBalancer Service, if needed
// Implementations must treat the *v1.Service and *v1.Node
// parameters as read-only and not modify them.
// Parameter 'clusterName' is the name of the cluster as presented to kube-controller-manager
func (lb *loadbalancer) UpdateLoadBalancer(ctx context.Context, clusterName string, service *corev1.Service, nodes []*corev1.Node) error {
	klog.Infof("UpdateLoadBalancer for service %s", service.GetName())
	lbService, err := lb.fetchVpcLoadbalancerFromCloud(ctx, clusterName, service)
	if err != nil {
		return fmt.Errorf("failed to get LoadBalancer service: %v", err)
	}
	if lbService == nil {
		return fmt.Errorf("LoadBalancer not found in Cloud API for service %s", service.GetName())
	}

	if _, err := lb.updateVpcLoadbalancerListenersAndTargetGroups(ctx, clusterName, service, nodes, lbService); err != nil {
		return fmt.Errorf("failed to update loadbalancer listeners and target groups: %v", err)
	}
	return nil
}

func (lb *loadbalancer) EnsureLoadBalancerDeleted(ctx context.Context, clusterName string, service *corev1.Service) error {
	klog.Infof("EnsureLoadBalancerDeleted for service %s", service.GetName())
	vpcLoadbalancer, err := lb.fetchVpcLoadbalancerFromCloud(ctx, clusterName, service)
	if err != nil {
		klog.Errorf("Failed to get LoadBalancer service: %v", err)
		return err
	}
	if vpcLoadbalancer != nil {

		// make sure we delete all target groups first
		if err = lb.cleanupUnusedTargetGroups(ctx, service, vpcLoadbalancer, nil); err != nil {
			klog.Errorf("Failed to cleanup unused target groups: %v", err)
			return err
		}

		if err = lb.iaasClient.DeleteLoadbalancer(ctx, vpcLoadbalancer.Identity); err != nil {
			klog.Errorf("Failed to delete LoadBalancer service: %v", err)
			return err
		}

		// wait until LB is deleted
		err = wait.PollUntilContextTimeout(ctx, lb.getLoadBalancerCreatePollInterval(), lb.getLoadBalancerCreatePollTimeout(), true, func(ctx context.Context) (bool, error) {
			vpclb, err := lb.fetchVpcLoadbalancerFromCloud(ctx, clusterName, service)
			if err != nil {
				return false, nil
			}
			if vpclb == nil {
				return true, nil
			}
			return false, nil
		})
		if err != nil {
			klog.Errorf("Failed to wait for LoadBalancer service to be deleted: %v", err)
			return err
		}

		// list all target groups and delete them
		targetGroups, err := lb.iaasClient.ListTargetGroups(ctx, &iaas.ListTargetGroupsRequest{
			Filters: []filters.Filter{
				&filters.LabelFilter{
					MatchLabels: lb.GetLabelsForVpcLoadbalancer(service),
				},
			},
		})
		if err != nil {
			klog.Errorf("Failed to list target groups: %v", err)
			return err
		}
		for _, targetGroup := range targetGroups {
			if len(targetGroup.LoadbalancerListeners) > 0 {
				klog.Infof("target group %q has listeners, skipping", targetGroup.Identity)
				continue
			}

			if err = lb.iaasClient.DeleteTargetGroup(ctx, iaas.DeleteTargetGroupRequest{
				Identity: targetGroup.Identity,
			}); err != nil {
				klog.Errorf("Failed to delete target group: %v", err)
				return err
			}
		}

	}
	return nil
}

func (lb *loadbalancer) fetchVpcLoadbalancerFromCloud(ctx context.Context, clusterName string, service *corev1.Service) (*iaas.VpcLoadbalancer, error) {
	loadbalancersInVpc, err := lb.iaasClient.ListLoadbalancers(ctx, &iaas.ListLoadbalancersRequest{
		Filters: []filters.Filter{
			&filters.FilterKeyValue{
				Key:   "vpc",
				Value: lb.vpcIdentity,
			},
			// 	{
			// 		Key:   "name",
			// 		Value: lbName,
			// 	},
		},
	})
	if err != nil {
		return nil, err
	}

	if len(loadbalancersInVpc) == 0 {
		klog.V(4).Infof("no loadbalancers found in vpc %q", lb.vpcIdentity)
		return nil, nil
	}

	labels := lb.GetLabelsForVpcLoadbalancer(service)
	for _, loadbalancer := range loadbalancersInVpc {
		if !matchLabels(labels, loadbalancer.Labels) {
			klog.V(6).Infof("loadbalancer %q has different labels than expected, skipping (expected: %v, actual: %v)", loadbalancer.Identity, labels, loadbalancer.Labels)
			continue
		}
		klog.V(4).Infof("loadbalancer %q has matching labels, returning", loadbalancer.Identity)
		return &loadbalancer, nil
	}

	klog.V(4).Infof("warning: no loadbalancer found in vpc %q with matching labels, trying to find by name", lb.vpcIdentity)

	// fallback to use name?
	lbName := lb.GetLoadBalancerName(ctx, clusterName, service)
	for _, loadbalancer := range loadbalancersInVpc {
		if loadbalancer.Name == lbName {
			klog.V(4).Infof("loadbalancer %q has matching name, returning", loadbalancer.Identity)
			return &loadbalancer, nil
		}
	}

	return nil, nil
}

func matchLabels(expectedLabels map[string]string, actualLabels map[string]string) bool {
	for k, v := range expectedLabels {
		if actualLabels[k] != v {
			return false
		}
	}
	return true
}

func (lb *loadbalancer) getSubnetIdentityForService(service *corev1.Service) string {
	if val, ok := service.Annotations[LoadBalancerAnnotationSubnetID]; ok && val != "" {
		return val
	}
	return lb.defaultSubnet
}

func (lb *loadbalancer) createVpcLoadbalancer(ctx context.Context, lbName string, service *corev1.Service) (*iaas.VpcLoadbalancer, error) {
	// find the vpc
	vpc, err := lb.iaasClient.GetVpc(ctx, lb.vpcIdentity)
	if err != nil {
		return nil, fmt.Errorf("failed to get vpc: %v", err)
	}

	if len(vpc.Subnets) == 0 {
		return nil, fmt.Errorf("vpc %s has no subnets", lb.vpcIdentity)
	}

	var vpcSubnet *iaas.Subnet
	requestedSubnetIdentity := lb.getSubnetIdentityForService(service)
	if requestedSubnetIdentity != "" {
		for _, subnet := range vpc.Subnets {
			if subnet.Identity == requestedSubnetIdentity || subnet.Slug == requestedSubnetIdentity {
				vpcSubnet = &subnet
				break
			}
		}
	} else {
		if len(vpc.Subnets) == 0 {
			return nil, fmt.Errorf("vpc %s has no subnets", lb.vpcIdentity)
		}
		vpcSubnet = ptr.To(vpc.Subnets[0])
	}
	if vpcSubnet == nil {
		return nil, fmt.Errorf("no subnet found for deploying loadbalancer for service %s", service.GetName())
	}

	// parse the annotations from the service
	// serverTimeout := int32(6000)
	// if val, ok := service.Annotations[LoadbalancerAnnotationServerTimeout]; ok {
	// 	timeout, err := strconv.ParseInt(val, 0, 32)
	// 	if err != nil {
	// 		klog.Errorf("failed to parse server-timeout annotation: %v", err)
	// 		return nil, fmt.Errorf("failed to parse server-timeout annotation: %v", err)
	// 	}
	// 	serverTimeout = int32(timeout)
	// }

	// clientUsageTimeout := int32(6000)
	// if val, ok := service.Annotations[LoadbalancerAnnotationClientTimeout]; ok {
	// 	timeout, err := strconv.ParseInt(val, 0, 32)
	// 	if err != nil {
	// 		klog.Errorf("failed to parse client-timeout annotation: %v", err)
	// 		return nil, fmt.Errorf("failed to parse client-timeout annotation: %v", err)
	// 	}
	// 	clientUsageTimeout = int32(timeout)
	// }

	// connectTimeout := int32(10000)
	// if val, ok := service.Annotations[LoadbalancerAnnotationConnectTimeout]; ok {
	// 	timeout, err := strconv.ParseInt(val, 0, 32)
	// 	if err != nil {
	// 		klog.Errorf("failed to parse connect-timeout annotation: %v", err)
	// 		return nil, fmt.Errorf("failed to parse connect-timeout annotation: %v", err)
	// 	}
	// 	connectTimeout = int32(timeout)
	// }

	internalLoadbalancer := false
	if val, ok := service.Annotations[LoadbalancerAnnotationInternal]; ok {
		internalLoadbalancer, _ = strconv.ParseBool(val)
	}

	labels := lb.GetLabelsForVpcLoadbalancer(service)
	annotations := lb.GetAnnotationsForVpcLoadbalancer(service)

	createLB := iaas.CreateLoadbalancer{
		Name:        lbName,
		Description: fmt.Sprintf("Loadbalancer for Kubernetes service %s", service.GetName()),
		Labels:      labels,
		Annotations: annotations,

		Subnet:                   vpcSubnet.Identity,
		InternalLoadbalancer:     internalLoadbalancer,
		SecurityGroupAttachments: []string{},
	}
	created, err := lb.iaasClient.CreateLoadbalancer(ctx, createLB)
	if err != nil {
		klog.Errorf("Failed to create vpc loadbalancer %s: %v", lbName, err)
		return nil, err
	}

	return created, nil
}

func getPortName(lbName string, port corev1.ServicePort) string {
	if port.Name != "" {
		return fmt.Sprintf("%s-%s", lbName, port.Name)
	}
	return fmt.Sprintf("%s-%s-p%d", lbName, strings.ToLower(string(port.Protocol)), port.Port)
}

func (lb *loadbalancer) getLoadBalancerCreatePollInterval() time.Duration {
	return convertLoadBalancerCreatePollConfig(lb.config.CreationPollInterval, defaultLoadBalancerCreatePollInterval, "interval")
}

func (lb *loadbalancer) getLoadBalancerCreatePollTimeout() time.Duration {
	return convertLoadBalancerCreatePollConfig(lb.config.CreationPollTimeout, defaultLoadBalancerCreatePollTimeout, "timeout")
}

func convertLoadBalancerCreatePollConfig(configValue *int, defaultValue time.Duration, name string) time.Duration {
	if configValue == nil {
		klog.Infof("setting creation poll %s to default value '%d'", name, defaultValue)
		return defaultValue
	}
	if *configValue <= 0 {
		klog.Warningf("creation poll %s %d must be > 0. Setting to '%d'", name, *configValue, defaultValue)
		return defaultValue
	}
	duration := time.Duration(*configValue) * time.Second
	if duration <= 0 {
		klog.Warningf("computed duration for creation poll %s is non-positive. Setting to default value '%d'", name, defaultValue)
		return defaultValue
	}
	return duration
}

func (lb *loadbalancer) updateVpcLoadbalancerListenersAndTargetGroups(ctx context.Context, clusterName string, service *corev1.Service, nodes []*corev1.Node, vpcLoadbalancer *iaas.VpcLoadbalancer) (*corev1.LoadBalancerStatus, error) {
	if vpcLoadbalancer == nil {
		klog.Infof("no load balancer provided during update, fetching from cloud")
		lb, err := lb.fetchVpcLoadbalancerFromCloud(ctx, clusterName, service)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch loadbalancer from cloud: %v", err)
		}
		vpcLoadbalancer = lb
		if vpcLoadbalancer == nil {
			klog.Errorf("loadbalancer not found in cloud")
			return nil, fmt.Errorf("loadbalancer not found in cloud")
		}
	}

	desiredListeners := lb.desiredVpcLoadbalancerListener(service)
	desiredTgs, err := lb.getDesiredVpcLoadbalancerTargetGroups(service, nodes)
	if err != nil {
		return nil, fmt.Errorf("failed to create loadbalancer backends: %v", err)
	}

	tgs, err := lb.createOrUpdateTargetGroups(ctx, service, vpcLoadbalancer, desiredTgs, nodes)
	if err != nil {
		return nil, fmt.Errorf("failed to create or update target groups: %v", err)
	}
	// update listeners
	if err := lb.updateVpcLoadbalancerListener(ctx, service, vpcLoadbalancer, desiredListeners, tgs); err != nil {
		return nil, fmt.Errorf("failed to update loadbalancer listener: %v", err)
	}
	// clean-up target groups that are not in the desired state
	if err := lb.cleanupUnusedTargetGroups(ctx, service, vpcLoadbalancer, tgs); err != nil {
		return nil, fmt.Errorf("failed to cleanup unused target groups: %v", err)
	}

	// update the loadbalancer itself if necessary
	// if err := lb.updateVpcLoadbalancer(ctx, vpcLoadbalancer); err != nil {
	// 	return nil, fmt.Errorf("failed to update loadbalancer: %v", err)
	// }

	loadbalancerStatus := &corev1.LoadBalancerStatus{
		Ingress: []corev1.LoadBalancerIngress{},
	}
	for _, ip := range vpcLoadbalancer.ExternalIpAddresses {
		if ip != "" {
			loadbalancerStatus.Ingress = append(loadbalancerStatus.Ingress, corev1.LoadBalancerIngress{
				IP:       ip,
				Hostname: vpcLoadbalancer.Hostname,
				IPMode:   ptr.To(corev1.LoadBalancerIPModeProxy),
			})
		}
	}
	return loadbalancerStatus, nil
}
