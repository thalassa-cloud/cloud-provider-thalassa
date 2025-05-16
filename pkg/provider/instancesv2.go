package provider

import (
	"context"
	"fmt"
	"regexp"

	"github.com/thalassa-cloud/client-go/filters"
	"github.com/thalassa-cloud/client-go/iaas"
	thalassaclient "github.com/thalassa-cloud/client-go/pkg/client"

	corev1 "k8s.io/api/core/v1"
	cloudprovider "k8s.io/cloud-provider"
	v1helper "k8s.io/cloud-provider/node/helpers"
	"k8s.io/klog/v2"
)

// Must match providerIDs built by cloudprovider.GetInstanceProviderID
var providerIDRegexp = regexp.MustCompile(`^` + ProviderName + `://([0-9A-Za-z_-]+)$`)

type instancesV2 struct {
	config *InstancesV2Config

	iaasClient *iaas.Client

	additionalLabels map[string]string
	cluster          string
	vpcIdentity      string
	defaultSubnet    string
}

// InstanceExists returns true if the instance for the given node exists according to the cloud provider.
func (i *instancesV2) InstanceExists(ctx context.Context, node *corev1.Node) (bool, error) {
	instanceID, err := instanceIDFromProviderID(node.Spec.ProviderID)
	if err != nil {
		return false, err
	}
	vmi, err := i.iaasClient.GetMachine(ctx, instanceID)
	if err != nil {
		return false, err
	}
	if vmi == nil {
		return false, nil
	}
	return true, nil
}

// InstanceShutdown returns true if the instance is shutdown according to the cloud provider.
func (i *instancesV2) InstanceShutdown(ctx context.Context, node *corev1.Node) (bool, error) {
	instanceID, err := instanceIDFromProviderID(node.Spec.ProviderID)
	if err != nil {
		return false, err
	}

	vmi, err := i.iaasClient.GetMachine(ctx, instanceID)
	if err != nil {
		if thalassaclient.IsNotFound(err) {
			return true, nil
		}
		return false, err
	}

	switch vmi.Status.Status {
	case "deleted":
		klog.Infof("instance %s is shutdown.", vmi.Name)
		return true, nil
	case "unknown":
		return true, fmt.Errorf("instance is in unknown state")
	default:
		return false, nil
	}
}

// InstanceMetadata returns the instance's metadata.
func (i *instancesV2) InstanceMetadata(ctx context.Context, node *corev1.Node) (*cloudprovider.InstanceMetadata, error) {
	virtualMachineInstance, err := i.findVirtualMachine(ctx, node)
	if err != nil {
		return nil, err
	}
	nodeAddresses := i.getNodeAddresses(virtualMachineInstance, node.Status.Addresses)

	region, zone := "", ""
	// find the vpc
	vpc, err := i.iaasClient.GetVpc(ctx, i.vpcIdentity)
	if err != nil {
		return nil, err
	}
	region = vpc.CloudRegion.Slug

	if virtualMachineInstance.AvailabilityZone != nil {
		zone = *virtualMachineInstance.AvailabilityZone
	}

	additionalLabels := map[string]string{}
	return &cloudprovider.InstanceMetadata{
		ProviderID:       getProviderID(virtualMachineInstance.Identity),
		NodeAddresses:    nodeAddresses,
		InstanceType:     i.getInstanceType(virtualMachineInstance),
		Region:           region,
		Zone:             zone,
		AdditionalLabels: additionalLabels,
	}, nil
}

func (*instancesV2) getInstanceType(instance *iaas.Machine) string {
	if instance.MachineType != nil {
		return instance.MachineType.Slug
	}
	return ""
}

// findVirtualMachine finds a virtual machine instance of the corresponding node
func (i *instancesV2) findVirtualMachine(ctx context.Context, node *corev1.Node) (*iaas.Machine, error) {
	// TODO: implement filters in the API
	machines, err := i.iaasClient.ListMachines(ctx, &iaas.ListMachinesRequest{
		Filters: []filters.Filter{
			&filters.FilterKeyValue{
				Key:   "vpc",
				Value: i.vpcIdentity,
			},
			// &filters.LabelFilter{
			// 	MatchLabels: map[string]string{
			// 		"name": node.GetName(),
			// 	},
			// },
		},
	})
	if err != nil {
		return nil, err
	}
	for _, machine := range machines {
		if machine.Vpc == nil {
			continue
		}
		if machine.Vpc.Identity != i.vpcIdentity {
			continue
		}
		if machine.Slug == node.GetName() {
			return &machine, nil
		}
	}
	return nil, cloudprovider.InstanceNotFound
}

func (i *instancesV2) getNodeAddresses(vmi *iaas.Machine, prevAddrs []corev1.NodeAddress) []corev1.NodeAddress {
	var addrs []corev1.NodeAddress
	foundInternalIP := false
	for _, i := range vmi.Interfaces {
		// TODO: do we handle IPv6 correctly here?
		if i.Name == "default" && len(i.IPAddresses) > 0 {
			for _, ip := range i.IPAddresses {
				v1helper.AddToNodeAddresses(&addrs, corev1.NodeAddress{
					Type:    corev1.NodeInternalIP,
					Address: ip,
				})
			}
			foundInternalIP = true
			break
		}
	}

	// fall back to the previously known internal IP on the node
	if !foundInternalIP {
		for _, prevAddr := range prevAddrs {
			if prevAddr.Type == corev1.NodeInternalIP {
				v1helper.AddToNodeAddresses(&addrs, prevAddr)
			}
		}
	}
	return addrs
}

func getProviderID(machineIdentity string) string {
	return fmt.Sprintf("%s://%s", ProviderName, machineIdentity)
}

// instanceIDFromProviderID extracts the instance ID from a provider ID.
func instanceIDFromProviderID(providerID string) (instanceID string, err error) {
	matches := providerIDRegexp.FindStringSubmatch(providerID)
	if len(matches) != 2 {
		return "", fmt.Errorf("mismatched ProviderID \"%s\" didn't match expected format \"%s://<instance-id>\"", providerID, ProviderName)
	}
	return matches[1], nil
}
