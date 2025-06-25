package provider

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/thalassa-cloud/client-go/iaas"
	"github.com/thalassa-cloud/client-go/pkg/client"
	"gopkg.in/yaml.v2"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	cloudprovider "k8s.io/cloud-provider"
	"k8s.io/utils/ptr"

	versionpkg "github.com/thalassa-cloud/cloud-provider-thalassa/pkg/version"
)

const (
	// ProviderName is the name of the Thalassa Cloud provider
	ProviderName = "thalassacloud"
)

var scheme = runtime.NewScheme()

func init() {
	cloudprovider.RegisterCloudProvider(ProviderName, thalassaCloudProviderFactory)
	if err := corev1.AddToScheme(scheme); err != nil {
		panic(err)
	}
}

type Cloud struct {
	config CloudConfig

	iaasClient *iaas.Client
}

type CloudConfig struct {
	InstancesV2  InstancesV2Config  `yaml:"instancesV2"`
	LoadBalancer LoadBalancerConfig `yaml:"loadBalancer"`

	Organisation     string           `yaml:"organisation"`
	Project          string           `yaml:"project"`
	Endpoint         string           `yaml:"endpoint"`
	Insecure         bool             `yaml:"insecure"`
	CloudCredentials CloudCredentials `yaml:"cloudCredentials"`

	VpcIdentity      string            `yaml:"vpcIdentity"`
	DefaultSubnet    string            `yaml:"defaultSubnet"`
	Cluster          string            `yaml:"cluster"`
	AdditionalLabels map[string]string `yaml:"additionalLabels"`
}

type CloudCredentials struct {
	PersonalAccessToken string `yaml:"personalAccessToken,omitempty"`
	ClientID            string `yaml:"clientID,omitempty"`
	ClientSecret        string `yaml:"clientSecret,omitempty"`
}

type LoadBalancerConfig struct {
	// Enabled activates the load balancer interface of the CCM
	Enabled bool `yaml:"enabled"`

	// CreationPollInterval determines how many seconds to wait for the load balancer creation between retries
	CreationPollInterval *int `yaml:"creationPollInterval,omitempty"`

	// CreationPollTimeout determines how many seconds to wait for the load balancer creation
	CreationPollTimeout *int `yaml:"creationPollTimeout,omitempty"`
}

type InstancesV2Config struct {
	// Enabled activates the instances interface of the CCM
	Enabled bool `yaml:"enabled"`
	// ZoneAndRegionEnabled indicates if need to get Region and zone labels from the cloud provider
	ZoneAndRegionEnabled bool `yaml:"zoneAndRegionEnabled"`
}

// createDefaultCloudConfig creates a CloudConfig object filled with default values.
// These default values should be overwritten by values read from the cloud-config file.
func createDefaultCloudConfig() CloudConfig {
	return CloudConfig{
		LoadBalancer: LoadBalancerConfig{
			Enabled:              true,
			CreationPollInterval: ptr.To(int(defaultLoadBalancerCreatePollInterval.Seconds())),
			CreationPollTimeout:  ptr.To(int(defaultLoadBalancerCreatePollTimeout.Seconds())),
		},
		InstancesV2: InstancesV2Config{
			Enabled:              true,
			ZoneAndRegionEnabled: true,
		},
	}
}

func NewCloudConfigFromBytes(configBytes []byte) (CloudConfig, error) {
	var config = createDefaultCloudConfig()
	err := yaml.Unmarshal(configBytes, &config)
	if err != nil {
		return CloudConfig{}, err
	}
	return config, nil
}

func thalassaCloudProviderFactory(config io.Reader) (cloudprovider.Interface, error) {
	if config == nil {
		return nil, fmt.Errorf("no %s cloud provider config file given", ProviderName)
	}

	buf := new(bytes.Buffer)
	_, err := buf.ReadFrom(config)
	if err != nil {
		return nil, fmt.Errorf("failed to read cloud provider config: %v", err)
	}
	cloudConf, err := NewCloudConfigFromBytes(buf.Bytes())
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal cloud provider config: %v", err)
	}
	tokenURL := fmt.Sprintf("%s/oidc/token", cloudConf.Endpoint)

	// TODO: construct the thalassa client
	opts := []client.Option{
		client.WithBaseURL(cloudConf.Endpoint),
		client.WithOrganisation(cloudConf.Organisation),
		client.WithUserAgent(fmt.Sprintf("cloud-provider-thalassa/%s", versionpkg.Version())),
	}
	if cloudConf.Project != "" {
		opts = append(opts, client.WithProject(cloudConf.Project))
	}
	if cloudConf.Insecure {
		opts = append(opts, client.WithInsecure())
	}
	if cloudConf.CloudCredentials.PersonalAccessToken != "" {
		opts = append(opts, client.WithAuthPersonalToken(cloudConf.CloudCredentials.PersonalAccessToken))
	}
	if cloudConf.CloudCredentials.ClientID != "" && cloudConf.CloudCredentials.ClientSecret != "" {
		if cloudConf.Insecure {
			opts = append(opts, client.WithAuthOIDCInsecure(cloudConf.CloudCredentials.ClientID, cloudConf.CloudCredentials.ClientSecret, tokenURL, cloudConf.Insecure))
		} else {
			opts = append(opts, client.WithAuthOIDC(cloudConf.CloudCredentials.ClientID, cloudConf.CloudCredentials.ClientSecret, tokenURL))
		}
	}

	thalassaClient, err := client.NewClient(opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create thalassa client: %v", err)
	}

	iaasClient, err := iaas.New(thalassaClient)
	if err != nil {
		return nil, fmt.Errorf("failed to create iaas client: %v", err)
	}

	// test access
	vpc, err := iaasClient.GetVpc(context.Background(), cloudConf.VpcIdentity)
	if err != nil {
		if client.IsNotFound(err) {
			return nil, fmt.Errorf("vpc %s not found", cloudConf.VpcIdentity)
		}
		return nil, fmt.Errorf("failed to test access to thalassa: %v", err)
	}
	if vpc == nil {
		return nil, fmt.Errorf("invalid response from thalassa: vpc %s not found", cloudConf.VpcIdentity)
	}

	if cloudConf.DefaultSubnet == "" {
		subnets := vpc.Subnets
		if len(subnets) == 0 {
			return nil, fmt.Errorf("no subnets found for vpc %s to discover the default subnet", cloudConf.VpcIdentity)
		}
		if len(subnets) > 1 {
			// find the subnet with the label "kubernetes.io/role/lb"
			cloudConf.DefaultSubnet, err = discoverDefaultSubnet(subnets)
			if err != nil {
				return nil, err
			}
		} else {
			cloudConf.DefaultSubnet = subnets[0].Identity
		}
	}

	return &Cloud{
		config:     cloudConf,
		iaasClient: iaasClient,
	}, nil
}

// Initialize provides the Cloud with a kubernetes client builder and may spawn goroutines
// to perform housekeeping activities within the Cloud provider.
func (c *Cloud) Initialize(clientBuilder cloudprovider.ControllerClientBuilder, stop <-chan struct{}) {
}

// LoadBalancer returns a balancer interface. Also returns true if the interface is supported, false otherwise.
func (c *Cloud) LoadBalancer() (cloudprovider.LoadBalancer, bool) {
	if !c.config.LoadBalancer.Enabled {
		return nil, false
	}
	return &loadbalancer{
		iaasClient: c.iaasClient,

		config:           c.config.LoadBalancer,
		additionalLabels: c.config.AdditionalLabels,

		vpcIdentity:   c.config.VpcIdentity,
		defaultSubnet: c.config.DefaultSubnet,
		cluster:       c.config.Cluster,
	}, true
}

// Instances returns an instances interface. Also returns true if the interface is supported, false otherwise.
func (c *Cloud) Instances() (cloudprovider.Instances, bool) {
	return nil, false
}

func (c *Cloud) InstancesV2() (cloudprovider.InstancesV2, bool) {
	if !c.config.InstancesV2.Enabled {
		return nil, false
	}
	return &instancesV2{
		iaasClient: c.iaasClient,

		config:           &c.config.InstancesV2,
		additionalLabels: c.config.AdditionalLabels,

		vpcIdentity:   c.config.VpcIdentity,
		defaultSubnet: c.config.DefaultSubnet,
		cluster:       c.config.Cluster,
	}, true
}

// Zones returns a zones interface. Also returns true if the interface is supported, false otherwise.
// DEPRECATED: Zones is deprecated in favor of retrieving zone/region information from InstancesV2.
func (c *Cloud) Zones() (cloudprovider.Zones, bool) {
	return nil, false
}

// Clusters returns a clusters interface.  Also returns true if the interface is supported, false otherwise.
func (c *Cloud) Clusters() (cloudprovider.Clusters, bool) {
	return nil, false
}

// Routes returns a routes interface along with whether the interface is supported.
func (c *Cloud) Routes() (cloudprovider.Routes, bool) {
	return nil, false
}

// ProviderName returns the Cloud provider ID.
func (c *Cloud) ProviderName() string {
	return ProviderName
}

// HasClusterID returns true if a ClusterID is required and set
func (c *Cloud) HasClusterID() bool {
	return c.config.Cluster != ""
}

func (c *Cloud) GetCloudConfig() CloudConfig {
	return c.config
}

func discoverDefaultSubnet(subnets []iaas.Subnet) (string, error) {
	for _, subnet := range subnets {
		role, ok := subnet.Labels["kubernetes.io/role/lb"]
		if !ok {
			continue
		}
		switch strings.ToLower(role) {
		case "true", "1", "yes":
			return subnet.Identity, nil
		}
	}
	return "", fmt.Errorf("no subnet found with the label 'kubernetes.io/role/lb'")
}
