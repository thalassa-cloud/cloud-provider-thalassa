package provider

import (
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
)

// GetLabelsForVpcLoadbalancer returns the labels for the VPC Loadbalancer
// The labels are used to identify the loadbalancer in the VPC and are used to link the loadbalancer to the service
func (lb *loadbalancer) GetLabelsForVpcLoadbalancer(service *corev1.Service) map[string]string {
	labels := map[string]string{
		"k8s.thalassa.cloud/kubernetes-cluster":           lb.cluster,
		"k8s.thalassa.cloud/cloud-provider-managed":       "true",
		"k8s.thalassa.cloud/kubernetes-service-name":      service.GetName(),
		"k8s.thalassa.cloud/kubernetes-service-namespace": service.GetNamespace(),
		"k8s.thalassa.cloud/kubernetes-service-uid":       string(service.UID),
	}

	for key, val := range lb.additionalLabels {
		if _, ok := labels[key]; !ok {
			labels[key] = val
		}
	}
	return labels
}

func (lb *loadbalancer) GetAnnotationsForVpcLoadbalancer(service *corev1.Service) map[string]string {
	annotations := map[string]string{}
	return annotations
}

func (lb *loadbalancer) GetLabelsForVpcLoadbalancerTargetGroup(service *corev1.Service, port int, protocol string) map[string]string {
	labels := lb.GetLabelsForVpcLoadbalancer(service)
	portLabels := map[string]string{
		"k8s.thalassa.cloud/kubernetes-service-port":     fmt.Sprintf("%d", port),
		"k8s.thalassa.cloud/kubernetes-service-protocol": strings.ToLower(protocol),
	}
	for key, val := range portLabels {
		if _, ok := labels[key]; !ok {
			labels[key] = val
		}
	}
	return labels
}
