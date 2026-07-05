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
		if thalassaclient.IsNotFound(err) {
			return false, nil
		}
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

	return machineIsShutdown(vmi)
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

// machineIsShutdown reports whether a machine should be treated as shut down by the node lifecycle controller.
func machineIsShutdown(machine *iaas.Machine) (bool, error) {
	if machine == nil {
		return true, nil
	}

	switch machine.State {
	case iaas.MachineStateRunning:
		return false, nil
	case iaas.MachineStateStopped:
		klog.Infof("instance %s is stopped.", machine.Name)
		return true, nil
	case iaas.MachineStateDeleting, iaas.MachineStateDeleted:
		klog.Infof("instance %s is deleted.", machine.Name)
		return true, nil
	}

	switch machine.Status.Status {
	case "deleted":
		klog.Infof("instance %s is shutdown.", machine.Name)
		return true, nil
	case "unknown":
		return true, fmt.Errorf("instance is in unknown state")
	default:
		return false, nil
	}
}

// findVirtualMachine finds a virtual machine instance of the corresponding node.
func (i *instancesV2) findVirtualMachine(ctx context.Context, node *corev1.Node) (*iaas.Machine, error) {
	if node.Spec.ProviderID != "" {
		instanceID, err := instanceIDFromProviderID(node.Spec.ProviderID)
		if err != nil {
			return nil, err
		}
		machine, err := i.iaasClient.GetMachine(ctx, instanceID)
		if err != nil {
			if thalassaclient.IsNotFound(err) {
				return nil, cloudprovider.InstanceNotFound
			}
			return nil, err
		}
		return machine, nil
	}

	machines, err := i.iaasClient.ListMachines(ctx, &iaas.ListMachinesRequest{
		Filters: []filters.Filter{
			&filters.FilterKeyValue{
				Key:   filters.FilterVpcIdentity,
				Value: i.vpcIdentity,
			},
			&filters.FilterKeyValue{
				Key:   filters.FilterSlug,
				Value: node.GetName(),
			},
		},
	})
	if err != nil {
		return nil, err
	}
	if len(machines) == 0 {
		return nil, cloudprovider.InstanceNotFound
	}
	return &machines[0], nil
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
