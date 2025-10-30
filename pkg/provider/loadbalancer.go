package provider

import (
	"context"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"time"

	"net"

	"github.com/thalassa-cloud/client-go/filters"
	"github.com/thalassa-cloud/client-go/iaas"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/util/workqueue"
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

	endpointSlicesClient clientset.Interface
	endpointSliceWatcher *EndpointSliceWatcher

	nodeFilter *NodeFilter

	// Queue for handling service resync requests
	serviceQueue workqueue.TypedRateLimitingInterface[string]

	// Context for managing goroutines
	ctx    context.Context
	cancel context.CancelFunc
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

	nodes, err = lb.nodeFilter.Filter(ctx, service, nodes)
	if err != nil {
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

	nodes, err = lb.nodeFilter.Filter(ctx, service, nodes)
	if err != nil {
		return err
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

		// delete managed security group if it exists
		lb.deleteManagedSecurityGroup(ctx, service)
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

	internalLoadbalancer := false
	if val, ok := service.Annotations[LoadbalancerAnnotationInternal]; ok {
		internalLoadbalancer, _ = strconv.ParseBool(val)
	}

	labels := lb.GetLabelsForVpcLoadbalancer(service)
	annotations := lb.GetAnnotationsForVpcLoadbalancer(service)

	securityGroups := lb.getSecurityGroupsForService(service)
	if err := lb.verifySecurityGroupsExist(ctx, securityGroups); err != nil {
		return nil, fmt.Errorf("failed to verify security groups: %v", err)
	}

	// Optionally create and attach a managed security group
	if lb.shouldCreateSecurityGroup(service) {
		sg, err := lb.ensureManagedSecurityGroup(ctx, service, lb.desiredVpcLoadbalancerListener(service))
		if err != nil {
			klog.Errorf("failed to ensure managed security group: %v", err)
			return nil, err
		}
		if sg != nil {
			securityGroups = append(securityGroups, sg.Identity)
		}
	}

	createLB := iaas.CreateLoadbalancer{
		Name:        lbName,
		Description: fmt.Sprintf("Loadbalancer for Kubernetes service %s", service.GetName()),
		Labels:      labels,
		Annotations: annotations,

		Subnet:                   vpcSubnet.Identity,
		InternalLoadbalancer:     internalLoadbalancer,
		SecurityGroupAttachments: securityGroups,
	}
	created, err := lb.iaasClient.CreateLoadbalancer(ctx, createLB)
	if err != nil {
		klog.Errorf("Failed to create vpc loadbalancer %s: %v", lbName, err)
		return nil, err
	}

	return created, nil
}

// verify security groups exists
func (lb *loadbalancer) verifySecurityGroupsExist(ctx context.Context, securityGroups []string) error {
	if len(securityGroups) == 0 { // no security groups to verify
		return nil
	}

	// list security groups in VPC
	securityGroupsInVpc, err := lb.iaasClient.ListSecurityGroups(ctx, &iaas.ListSecurityGroupsRequest{
		Filters: []filters.Filter{
			&filters.FilterKeyValue{
				Key:   "vpc",
				Value: lb.vpcIdentity,
			},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to list security groups in vpc: %v", err)
	}

	for _, securityGroup := range securityGroups {
		found := false
		for _, securityGroupInVpc := range securityGroupsInVpc {
			if securityGroupInVpc.Identity == securityGroup {
				found = true
				break
			}
		}

		if !found {
			return fmt.Errorf("security group %s does not exist in vpc %s", securityGroup, lb.vpcIdentity)
		}
	}
	return nil
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

func (lb *loadbalancer) getSecurityGroupsForService(service *corev1.Service) []string {
	if val, ok := service.Annotations[LoadBalancerAnnotationSecurityGroups]; ok {
		return strings.Split(val, ",")
	}
	return []string{}
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
	if err := lb.updateVpcLoadbalancer(ctx, service, vpcLoadbalancer, desiredListeners); err != nil {
		return nil, fmt.Errorf("failed to update loadbalancer: %v", err)
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
	return loadbalancerStatus, nil
}

func (lb *loadbalancer) updateVpcLoadbalancer(ctx context.Context, service *corev1.Service, vpcLoadbalancer *iaas.VpcLoadbalancer, desiredListeners []iaas.VpcLoadbalancerListener) error {
	desiredSecurityGroups := lb.getSecurityGroupsForService(service)
	if err := lb.verifySecurityGroupsExist(ctx, desiredSecurityGroups); err != nil {
		return fmt.Errorf("failed to verify security groups: %v", err)
	}

	// current security groups
	currentSecurityGroups := vpcLoadbalancer.SecurityGroups
	currentSecurityGroupIdentities := make([]string, 0, len(currentSecurityGroups))
	for _, securityGroup := range currentSecurityGroups {
		currentSecurityGroupIdentities = append(currentSecurityGroupIdentities, securityGroup.Identity)
	}

	// Reconcile managed security group if requested
	if lb.shouldCreateSecurityGroup(service) {
		sg, err := lb.ensureManagedSecurityGroup(ctx, service, desiredListeners)
		if err != nil {
			klog.Errorf("failed to ensure managed security group: %v", err)
			return fmt.Errorf("failed to ensure managed security group: %v", err)
		}
		if sg != nil {
			desiredSecurityGroups = append(desiredSecurityGroups, sg.Identity)
		}
		// } else {
		// 	// delete any managed security groups
		// 	lb.deleteManagedSecurityGroup(ctx, service)
	}

	preferredSubnetIdentity := lb.getSubnetIdentityForService(service)
	if preferredSubnetIdentity == "" {
		preferredSubnetIdentity = vpcLoadbalancer.Subnet.Identity
	}

	// check if security groups need to be updated
	// different identities, or different number of security groups
	if !reflect.DeepEqual(desiredSecurityGroups, currentSecurityGroupIdentities) || len(desiredSecurityGroups) != len(currentSecurityGroupIdentities) || vpcLoadbalancer.Subnet.Identity != preferredSubnetIdentity {
		klog.Infof("loadbalancer %s needs to be updated", vpcLoadbalancer.Identity)
		if _, err := lb.iaasClient.UpdateLoadbalancer(ctx, vpcLoadbalancer.Identity, iaas.UpdateLoadbalancer{
			Name:                     vpcLoadbalancer.Name,
			Description:              vpcLoadbalancer.Description,
			Labels:                   vpcLoadbalancer.Labels,
			Annotations:              vpcLoadbalancer.Annotations,
			Subnet:                   ptr.To(preferredSubnetIdentity),
			DeleteProtection:         vpcLoadbalancer.DeleteProtection,
			SecurityGroupAttachments: desiredSecurityGroups,
		}); err != nil {
			return fmt.Errorf("failed to update loadbalancer: %v", err)
		}
	}

	return nil
}

// triggerServiceResync adds a service to the resync queue
func (lb *loadbalancer) triggerServiceResync(serviceKey string) {
	klog.V(4).Infof("Triggering resync for service %s", serviceKey)
	lb.serviceQueue.Add(serviceKey)
}

// processServiceQueue processes the service resync queue
func (lb *loadbalancer) processServiceQueue() {
	for {
		select {
		case <-lb.ctx.Done():
			return
		default:
			serviceKey, shutdown := lb.serviceQueue.Get()
			if shutdown {
				return
			}

			lb.processServiceResync(serviceKey)
			lb.serviceQueue.Done(serviceKey)
		}
	}
}

// processServiceResync processes a single service resync
func (lb *loadbalancer) processServiceResync(serviceKey string) {
	// Parse service key (namespace/name)
	parts := strings.Split(serviceKey, "/")
	if len(parts) != 2 {
		klog.Errorf("Invalid service key format: %s", serviceKey)
		return
	}

	namespace, name := parts[0], parts[1]

	// Get the service from the API server
	svc, err := lb.endpointSlicesClient.CoreV1().Services(namespace).Get(lb.ctx, name, metav1.GetOptions{})
	if err != nil {
		klog.Errorf("Failed to get service %s: %v", serviceKey, err)
		return
	}

	// Check if this is a LoadBalancer service
	if svc.Spec.Type != corev1.ServiceTypeLoadBalancer {
		klog.V(4).Infof("Service %s is not a LoadBalancer service, skipping resync", serviceKey)
		return
	}

	// Get all nodes
	nodes, err := lb.endpointSlicesClient.CoreV1().Nodes().List(lb.ctx, metav1.ListOptions{})
	if err != nil {
		klog.Errorf("Failed to list nodes for service %s: %v", serviceKey, err)
		return
	}
	// filter out nodes that are not ready
	readyNodes := filterReadyNodes(nodes.Items)
	// Trigger load balancer update
	klog.Infof("Processing resync for service %s", serviceKey)
	if err := lb.UpdateLoadBalancer(lb.ctx, lb.cluster, svc, readyNodes); err != nil {
		klog.Errorf("Failed to update load balancer for service %s: %v", serviceKey, err)
		// Re-queue with backoff
		lb.serviceQueue.AddRateLimited(serviceKey)
		return
	}

	klog.Infof("Successfully processed resync for service %s", serviceKey)
}

func filterReadyNodes(nodes []corev1.Node) []*corev1.Node {
	readyNodes := make([]*corev1.Node, 0, len(nodes))
	for _, node := range nodes {
		if IsNodeReady(&node) {
			readyNodes = append(readyNodes, &node)
		}
	}
	return readyNodes
}

func IsNodeReady(node *corev1.Node) bool {
	if node == nil {
		return false
	}
	if node.Status.Conditions == nil {
		return false
	}
	for _, condition := range node.Status.Conditions {
		if condition.Type == corev1.NodeReady && condition.Status == corev1.ConditionStatus(corev1.ConditionTrue) {
			return true
		}
	}
	return false
}

// startServiceQueueProcessor starts the service queue processor goroutine
func (lb *loadbalancer) startServiceQueueProcessor() {
	go lb.processServiceQueue()
}

// stopServiceQueueProcessor stops the service queue processor
func (lb *loadbalancer) stopServiceQueueProcessor() {
	if lb.cancel != nil {
		lb.cancel()
	}
	lb.serviceQueue.ShutDown()
}

// cleanup performs cleanup when the loadbalancer is no longer needed
func (lb *loadbalancer) cleanup() {
	lb.stopServiceQueueProcessor()
}

// shouldCreateSecurityGroup returns true if the service requests a managed SG
func (lb *loadbalancer) shouldCreateSecurityGroup(service *corev1.Service) bool {
	if val, ok := service.Annotations[LoadBalancerAnnotationCreateSecurityGroup]; ok {
		b, _ := strconv.ParseBool(val)
		return b
	}
	return false
}

// ensureManagedSecurityGroup creates or updates a managed security group based on desired listeners and attaches it
func (lb *loadbalancer) ensureManagedSecurityGroup(ctx context.Context, service *corev1.Service, desiredListeners []iaas.VpcLoadbalancerListener) (*iaas.SecurityGroup, error) {
	// find existing SG by labels
	sg, err := lb.findManagedSecurityGroup(ctx, service)
	if err != nil {
		return nil, fmt.Errorf("failed to find managed security group: %v", err)
	}

	labels := lb.GetLabelsForVpcLoadbalancer(service)
	annotations := lb.GetAnnotationsForVpcLoadbalancer(service)

	ingress := lb.buildIngressRulesFromListeners(desiredListeners)
	egress := []iaas.SecurityGroupRule{
		// allow all outbound traffic
		{
			Name:          "allow-all-outbound",
			IPVersion:     iaas.SecurityGroupIPVersionIPv4,
			Protocol:      iaas.SecurityGroupRuleProtocolAll,
			Priority:      100,
			RemoteType:    iaas.SecurityGroupRuleRemoteTypeAddress,
			RemoteAddress: ptr.To("0.0.0.0/0"),
		},
		{
			Name:          "allow-all-outbound",
			IPVersion:     iaas.SecurityGroupIPVersionIPv6,
			Protocol:      iaas.SecurityGroupRuleProtocolAll,
			Priority:      110,
			RemoteType:    iaas.SecurityGroupRuleRemoteTypeAddress,
			RemoteAddress: ptr.To("::/0"),
		},
	}

	if sg == nil {
		// create
		name := lb.generateSecurityGroupName(service.GetName())
		create := iaas.CreateSecurityGroupRequest{
			Name:                  name,
			Description:           fmt.Sprintf("Security group for Kubernetes service %s", service.GetName()),
			Labels:                labels,
			Annotations:           annotations,
			VpcIdentity:           lb.vpcIdentity,
			AllowSameGroupTraffic: true,
			IngressRules:          ingress,
			EgressRules:           egress,
		}
		created, err := lb.iaasClient.CreateSecurityGroup(ctx, create)
		if err != nil {
			return nil, fmt.Errorf("failed to create managed security group: %v", err)
		}
		return created, nil
	}

	// update rules if differ
	update := iaas.UpdateSecurityGroupRequest{
		Name:                  sg.Name,
		Description:           sg.Description,
		Labels:                labels,
		Annotations:           annotations,
		ObjectVersion:         sg.ObjectVersion,
		AllowSameGroupTraffic: true,
		IngressRules:          ingress,
		EgressRules:           egress,
	}
	updated, err := lb.iaasClient.UpdateSecurityGroup(ctx, sg.Identity, update)
	if err != nil {
		return nil, fmt.Errorf("failed to update managed security group: %v", err)
	}
	return updated, nil
}

// findManagedSecurityGroup locates the SG for this service via labels
func (lb *loadbalancer) findManagedSecurityGroup(ctx context.Context, service *corev1.Service) (*iaas.SecurityGroup, error) {
	labels := lb.GetLabelsForVpcLoadbalancer(service)

	securityGroupsInVpc, err := lb.iaasClient.ListSecurityGroups(ctx, &iaas.ListSecurityGroupsRequest{
		Filters: []filters.Filter{
			&filters.FilterKeyValue{Key: "vpc", Value: lb.vpcIdentity},
			&filters.LabelFilter{
				MatchLabels: labels,
			},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list security groups in vpc: %v", err)
	}
	for _, sg := range securityGroupsInVpc {
		if matchLabels(labels, sg.Labels) {
			return &sg, nil
		}
	}
	return nil, nil
}

// buildIngressRulesFromListeners creates SG ingress rules for each listener and source
func (lb *loadbalancer) buildIngressRulesFromListeners(listeners []iaas.VpcLoadbalancerListener) []iaas.SecurityGroupRule {
	rules := make([]iaas.SecurityGroupRule, 0)
	priority := int32(100)
	for _, l := range listeners {
		for _, src := range l.AllowedSources {
			ipVer := iaas.SecurityGroupIPVersionIPv4
			if _, ipnet, err := net.ParseCIDR(src); err == nil {
				if ip := ipnet.IP; ip != nil && ip.To4() == nil {
					ipVer = iaas.SecurityGroupIPVersionIPv6
				}
			}
			proto := iaas.SecurityGroupRuleProtocolTCP
			if strings.ToLower(string(l.Protocol)) == "udp" {
				proto = iaas.SecurityGroupRuleProtocolUDP
			}
			rules = append(rules, iaas.SecurityGroupRule{
				Name:          fmt.Sprintf("%s-%d", strings.ToLower(string(l.Protocol)), l.Port),
				IPVersion:     ipVer,
				Protocol:      proto,
				Priority:      priority,
				RemoteType:    iaas.SecurityGroupRuleRemoteTypeAddress,
				RemoteAddress: ptr.To(src),
				PortRangeMin:  int32(l.Port),
				PortRangeMax:  int32(l.Port),
				Policy:        iaas.SecurityGroupRulePolicyAllow,
			})
		}
	}
	return rules
}

// generateSecurityGroupName returns a short name within API constraints
func (lb *loadbalancer) generateSecurityGroupName(lbName string) string {
	// Ensure <=16 chars; prefix sg-
	base := "sg-" + lbName
	if len(base) > 16 {
		return base[:16]
	}
	return base
}

// deleteManagedSecurityGroup removes the managed SG if present
func (lb *loadbalancer) deleteManagedSecurityGroup(ctx context.Context, service *corev1.Service) {
	sg, err := lb.findManagedSecurityGroup(ctx, service)
	if err != nil || sg == nil {
		return
	}
	if err := lb.iaasClient.DeleteSecurityGroup(ctx, sg.Identity); err != nil {
		klog.Errorf("failed to delete managed security group %s: %v", sg.Identity, err)
	}
}
