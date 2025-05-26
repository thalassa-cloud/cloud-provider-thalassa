package provider

import (
	"fmt"
	"strconv"

	"github.com/thalassa-cloud/client-go/iaas"
	corev1 "k8s.io/api/core/v1"
)

func GetIdleConnectionTimeout(service *corev1.Service) (int, error) {
	return getIntAnnotation(service, LoadbalancerAnnotationIdleConnectionTimeout, DefaultIdleConnectionTimeout)
}

func GetEnableProxyProtocol(service *corev1.Service) (bool, error) {
	return getBoolAnnotation(service, LoadbalancerAnnotationEnableProxyProtocol, DefaultEnableProxyProtocol)
}

func GetMaxConnections(service *corev1.Service) (int, error) {
	return getIntAnnotation(service, LoadbalancerAnnotationMaxConnections, DefaultMaxConnections)
}

func GetLoadbalancingPolicy(service *corev1.Service) (iaas.LoadbalancingPolicy, error) {
	policy, err := getStringAnnotation(service, LoadbalancerAnnotationLoadbalancingPolicy, DefaultLoadbalancingPolicy)
	if err != nil {
		return iaas.LoadbalancingPolicyRoundRobin, err
	}

	// Validate the policy value
	switch iaas.LoadbalancingPolicy(policy) {
	case iaas.LoadbalancingPolicyRoundRobin, iaas.LoadbalancingPolicyRandom, iaas.LoadbalancingPolicyMagLev:
		return iaas.LoadbalancingPolicy(policy), nil
	default:
		return iaas.LoadbalancingPolicyRoundRobin, fmt.Errorf("invalid loadbalancing policy: %s, must be one of: ROUND_ROBIN, RANDOM, MAGLEV", policy)
	}
}

func getStringAnnotation(service *corev1.Service, annotation string, defaultValue string) (string, error) {
	if val, ok := service.Annotations[annotation]; ok {
		return val, nil
	}
	return defaultValue, nil
}

func getIntAnnotation(service *corev1.Service, annotation string, defaultValue int) (int, error) {
	if val, ok := service.Annotations[annotation]; ok {
		c, err := strconv.Atoi(val)
		if err != nil {
			return defaultValue, fmt.Errorf("failed to parse %s: %v", annotation, err)
		}
		return c, nil
	}
	return defaultValue, nil
}

func getBoolAnnotation(service *corev1.Service, annotation string, defaultValue bool) (bool, error) {
	if val, ok := service.Annotations[annotation]; ok {
		enabled, err := strconv.ParseBool(val)
		if err != nil {
			return defaultValue, fmt.Errorf("failed to parse %s: %v", annotation, err)
		}
		return enabled, nil
	}
	return defaultValue, nil
}
