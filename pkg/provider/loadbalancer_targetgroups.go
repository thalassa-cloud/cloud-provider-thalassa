package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/thalassa-cloud/client-go/filters"
	"github.com/thalassa-cloud/client-go/iaas"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"
	"k8s.io/utils/ptr"
)

const (
	DefaultHealthCheckPath               = "/healthz"
	DefaultHealthCheckTimeoutSeconds     = 5
	DefaultHealthCheckPeriodSeconds      = 10
	DefaultHealthCheckHealthyThreshold   = 2
	DefaultHealthCheckUnhealthyThreshold = 3
	DefaultHealthCheckProtocol           = "http"
)

func (l *loadbalancer) getDesiredVpcLoadbalancerTargetGroups(service *corev1.Service, _ []*corev1.Node) ([]iaas.VpcLoadbalancerTargetGroup, error) {
	tgs := []iaas.VpcLoadbalancerTargetGroup{}

	enableProxyProtocol, err := getBoolAnnotation(service, LoadbalancerAnnotationEnableProxyProtocol, DefaultEnableProxyProtocol)
	if err != nil {
		klog.Errorf("failed to get enable proxy protocol: %v", err)
	}

	loadbalancingPolicy, err := GetLoadbalancingPolicy(service)
	if err != nil {
		klog.Errorf("failed to get loadbalancing policy: %v", err)
		return nil, err
	}

	healthCheckEnabled, err := getBoolAnnotation(service, LoadbalancerAnnotationHealthCheckEnabled, false)
	if err != nil {
		klog.Errorf("failed to get health check enabled: %v", err)
	}

	healthCheckPath, err := getStringAnnotation(service, LoadbalancerAnnotationHealthCheckPath, DefaultHealthCheckPath)
	if err != nil {
		klog.Errorf("failed to get health check path: %v", err)
	}
	healthCheckPort, err := getIntAnnotation(service, LoadbalancerAnnotationHealthCheckPort, -1)
	if err != nil {
		klog.Errorf("failed to get health check port: %v", err)
	}
	healthCheckProtocol, err := getStringAnnotation(service, LoadbalancerAnnotationHealthCheckProtocol, DefaultHealthCheckProtocol)
	if err != nil {
		klog.Errorf("failed to get health check protocol: %v", err)
	}
	healthCheckTimeoutSeconds, err := getIntAnnotation(service, LoadbalancerAnnotationHealthCheckTimeout, DefaultHealthCheckTimeoutSeconds)
	if err != nil {
		klog.Errorf("failed to get health check timeout seconds: %v", err)
	}
	healthCheckPeriodSeconds, err := getIntAnnotation(service, LoadbalancerAnnotationHealthCheckInterval, DefaultHealthCheckPeriodSeconds)
	if err != nil {
		klog.Errorf("failed to get health check period seconds: %v", err)
	}
	healthCheckHealthyThreshold, err := getIntAnnotation(service, LoadbalancerAnnotationHealthCheckUpThreshold, DefaultHealthCheckHealthyThreshold)
	if err != nil {
		klog.Errorf("failed to get health check healthy threshold: %v", err)
	}
	healthCheckUnhealthyThreshold, err := getIntAnnotation(service, LoadbalancerAnnotationHealthCheckDownThreshold, DefaultHealthCheckUnhealthyThreshold)
	if err != nil {
		klog.Errorf("failed to get health check unhealthy threshold: %v", err)
	}

	lbName := l.GetLoadBalancerName(context.Background(), l.cluster, service)

	for _, svcPort := range service.Spec.Ports {
		backend := iaas.VpcLoadbalancerTargetGroup{
			Name:                getPortName(lbName, svcPort),
			TargetPort:          int(svcPort.NodePort),
			Protocol:            iaas.LoadbalancerProtocol(strings.ToLower(string(svcPort.Protocol))),
			Labels:              l.GetLabelsForVpcLoadbalancerTargetGroup(service, int(svcPort.Port), string(svcPort.Protocol)),
			EnableProxyProtocol: ptr.To(enableProxyProtocol),
			LoadbalancingPolicy: &loadbalancingPolicy,

			// EnableHealthCheck: service.Spec.HealthCheckNodePort > 0, // TODO: implement health check
			// EnableStickySessions: enableStickySessions,
			// ServiceDiscovery:     "static",
			// HealthCheck:          healthCheck,
		}

		if service.Spec.HealthCheckNodePort > 0 {
			port := int32(service.Spec.HealthCheckNodePort)
			if healthCheckPort > 0 {
				port = int32(healthCheckPort)
			}
			backend.HealthCheck = &iaas.BackendHealthCheck{
				Port:               port,
				Protocol:           iaas.ProtocolHTTP,
				Path:               healthCheckPath,
				TimeoutSeconds:     healthCheckTimeoutSeconds,
				PeriodSeconds:      healthCheckPeriodSeconds,
				HealthyThreshold:   int32(healthCheckHealthyThreshold),
				UnhealthyThreshold: int32(healthCheckUnhealthyThreshold),
			}
		} else if healthCheckPort != -1 && healthCheckEnabled {
			backend.HealthCheck = &iaas.BackendHealthCheck{
				Port:               int32(healthCheckPort),
				Protocol:           iaas.LoadbalancerProtocol(healthCheckProtocol),
				Path:               healthCheckPath,
				TimeoutSeconds:     healthCheckTimeoutSeconds,
				PeriodSeconds:      healthCheckPeriodSeconds,
				HealthyThreshold:   int32(healthCheckHealthyThreshold),
				UnhealthyThreshold: int32(healthCheckUnhealthyThreshold),
			}
		}

		tgs = append(tgs, backend)
	}
	return tgs, nil
}

func (l *loadbalancer) cleanupUnusedTargetGroups(ctx context.Context, service *corev1.Service, _ *iaas.VpcLoadbalancer, desiredTargetGroups []iaas.VpcLoadbalancerTargetGroup) error {
	existingTargetGroups, err := l.iaasClient.ListTargetGroups(ctx, &iaas.ListTargetGroupsRequest{
		Filters: []filters.Filter{
			&filters.FilterKeyValue{
				Key:   filters.FilterVpcIdentity,
				Value: l.vpcIdentity,
			},
			&filters.LabelFilter{
				MatchLabels: l.GetLabelsForVpcLoadbalancer(service),
			},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to list target groups: %v", err)
	}

	desiredTargetGroupsMap := map[string]iaas.VpcLoadbalancerTargetGroup{}
	for _, targetGroup := range desiredTargetGroups {
		desiredTargetGroupsMap[fmt.Sprintf("%s:%d", targetGroup.Protocol, targetGroup.TargetPort)] = targetGroup
	}
	for _, targetGroup := range existingTargetGroups {
		if len(targetGroup.LoadbalancerListeners) > 0 {
			klog.Infof("target group %q has loadbalancer listeners, skipping", targetGroup.Identity)
			continue
		}

		if _, ok := desiredTargetGroupsMap[fmt.Sprintf("%s:%d", targetGroup.Protocol, targetGroup.TargetPort)]; !ok {
			if err := l.iaasClient.DeleteTargetGroup(ctx, iaas.DeleteTargetGroupRequest{Identity: targetGroup.Identity}); err != nil {
				return fmt.Errorf("failed to delete target group: %v", err)
			}
			klog.Infof("deleted target group %q", targetGroup.Identity)
		}
	}
	return nil
}

func (l *loadbalancer) createOrUpdateTargetGroups(ctx context.Context, service *corev1.Service, _ *iaas.VpcLoadbalancer, desiredTargetGroups []iaas.VpcLoadbalancerTargetGroup, nodes []*corev1.Node) ([]iaas.VpcLoadbalancerTargetGroup, error) {
	klog.Infof("creating or updating target groups for service %q", service.GetName())

	tgs := []iaas.VpcLoadbalancerTargetGroup{}

	existingTargetGroups, err := l.iaasClient.ListTargetGroups(ctx, &iaas.ListTargetGroupsRequest{
		Filters: []filters.Filter{
			&filters.FilterKeyValue{
				Key:   filters.FilterVpcIdentity,
				Value: l.vpcIdentity,
			},
			&filters.LabelFilter{
				MatchLabels: l.GetLabelsForVpcLoadbalancer(service),
			},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list target groups: %v", err)
	}
	// filter with the target group labels

	existingTargetGroupsMap := map[string]iaas.VpcLoadbalancerTargetGroup{}
	for _, targetGroup := range existingTargetGroups {
		existingTargetGroupsMap[fmt.Sprintf("%s:%d", targetGroup.Protocol, targetGroup.TargetPort)] = targetGroup
	}

	desiredTargetGroupsMap := map[string]iaas.VpcLoadbalancerTargetGroup{}
	for _, targetGroup := range desiredTargetGroups {
		desiredTargetGroupsMap[fmt.Sprintf("%s:%d", targetGroup.Protocol, targetGroup.TargetPort)] = targetGroup
	}

	klog.Infof("existing target groups: %d", len(existingTargetGroups))
	klog.Infof("desired target groups: %d", len(desiredTargetGroups))

	// create missing target groups
	for _, targetGroup := range desiredTargetGroups {
		if _, ok := existingTargetGroupsMap[fmt.Sprintf("%s:%d", targetGroup.Protocol, targetGroup.TargetPort)]; !ok {
			klog.Infof("creating target group %q", targetGroup.Name)
			created, err := l.iaasClient.CreateTargetGroup(ctx, iaas.CreateTargetGroup{
				Vpc:                 l.vpcIdentity,
				Name:                targetGroup.Name,
				Description:         targetGroup.Description,
				Protocol:            targetGroup.Protocol,
				TargetPort:          targetGroup.TargetPort,
				Labels:              targetGroup.Labels,
				Annotations:         targetGroup.Annotations,
				HealthCheck:         targetGroup.HealthCheck,
				EnableProxyProtocol: targetGroup.EnableProxyProtocol,
				LoadbalancingPolicy: targetGroup.LoadbalancingPolicy,
			})
			if err != nil {
				return nil, fmt.Errorf("failed to create target group: %v", err)
			}
			if created == nil {
				return nil, fmt.Errorf("failed to create target group: %v", err)
			}
			klog.Infof("created target group %q", created.Identity)

			tgs = append(tgs, *created)
			if err := l.upgradeTargetGroupAttachments(ctx, *created, nodes); err != nil {
				return nil, fmt.Errorf("failed to upgrade target group attachments: %v", err)
			}
		}
	}
	klog.Infof("completed creating new target groups, now updating existing target groups for service %q", service.GetName())

	// update existing target groups
	for _, targetGroup := range existingTargetGroups {
		desiredTargetGroup, ok := desiredTargetGroupsMap[fmt.Sprintf("%s:%d", targetGroup.Protocol, targetGroup.TargetPort)]
		if !ok {
			klog.Infof("target group %q not in desired target groups, skipping", targetGroup.Name)
			continue
		}

		if targetGroup.Identity == "" {
			klog.Infof("updating target group %q has no identity, skipping", targetGroup.Name)
			continue
		}

		klog.Infof("updating target group %q", targetGroup.Name)
		updated, err := l.iaasClient.UpdateTargetGroup(ctx, iaas.UpdateTargetGroupRequest{
			Identity: targetGroup.Identity,
			UpdateTargetGroup: iaas.UpdateTargetGroup{
				Name:                desiredTargetGroup.Name,
				Description:         desiredTargetGroup.Description,
				Protocol:            desiredTargetGroup.Protocol,
				TargetPort:          desiredTargetGroup.TargetPort,
				Labels:              desiredTargetGroup.Labels,
				Annotations:         desiredTargetGroup.Annotations,
				HealthCheck:         desiredTargetGroup.HealthCheck,
				EnableProxyProtocol: desiredTargetGroup.EnableProxyProtocol,
				LoadbalancingPolicy: desiredTargetGroup.LoadbalancingPolicy,
			},
		})
		if err != nil {
			return nil, fmt.Errorf("failed to update target group: %v", err)
		}
		if updated == nil {
			return nil, fmt.Errorf("failed to update target group: %v", err)
		}
		tgs = append(tgs, *updated)
		klog.Infof("updated target group %s", updated.Identity)
		if err := l.upgradeTargetGroupAttachments(ctx, *updated, nodes); err != nil {
			return nil, fmt.Errorf("failed to upgrade target group attachments: %v", err)
		}
	}

	klog.Infof("completed updating existing target groups for service %q", service.GetName())

	return tgs, nil
}

func (l *loadbalancer) upgradeTargetGroupAttachments(ctx context.Context, targetGroup iaas.VpcLoadbalancerTargetGroup, nodes []*corev1.Node) error {
	klog.Infof("upgrading target group attachments for target group %s with %d nodes", targetGroup.Identity, len(nodes))

	attachments := []iaas.AttachTarget{}
	for _, node := range nodes {
		providerId := node.Spec.ProviderID
		if providerId == "" {
			continue
		}
		providerIdParts := strings.Split(providerId, "://")
		if len(providerIdParts) != 2 {
			klog.Infof("failed to get provider ID for node %s", node.Name)
			continue
		}
		machineIdentity := providerIdParts[1]

		attachments = append(attachments, iaas.AttachTarget{
			ServerIdentity: machineIdentity,
		})
	}
	klog.Infof("attaching %d nodes to target group %s", len(attachments), targetGroup.Identity)

	// update the attachments
	if err := l.iaasClient.SetTargetGroupServerAttachments(ctx, iaas.TargetGroupAttachmentsBatch{
		TargetGroupID: targetGroup.Identity,
		Attachments:   attachments,
	}); err != nil {
		return fmt.Errorf("failed to update target group attachments: %v", err)
	}
	return nil
}
