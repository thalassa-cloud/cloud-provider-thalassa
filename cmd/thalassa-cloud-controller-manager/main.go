package main

import (
	"os"

	"k8s.io/apimachinery/pkg/util/wait"
	cloudprovider "k8s.io/cloud-provider"
	"k8s.io/cloud-provider/app"
	"k8s.io/cloud-provider/app/config"
	"k8s.io/cloud-provider/names"
	"k8s.io/cloud-provider/options"
	"k8s.io/component-base/cli"
	cliflag "k8s.io/component-base/cli/flag"
	_ "k8s.io/component-base/logs/json/register"
	_ "k8s.io/component-base/metrics/prometheus/clientgo"
	_ "k8s.io/component-base/metrics/prometheus/version"
	"k8s.io/klog/v2"

	_ "github.com/thalassa-cloud/cloud-provider-thalassa/pkg/provider"
	versionpkg "github.com/thalassa-cloud/cloud-provider-thalassa/pkg/version"
)

// these must be set by the compiler using LDFLAGS
// -X main.version= -X main.commit= -X main.date= -X main.builtBy=
var (
	version = "unknown"
	commit  = "unknown"
	date    = "unknown"
	builtBy = "go"
)

func main() {
	ccmOptions, err := options.NewCloudControllerManagerOptions()
	if err != nil {
		klog.Fatalf("unable to initialize command options: %v", err)
	}

	flagSet := cliflag.NamedFlagSets{}
	command := app.NewCloudControllerManagerCommand(ccmOptions, cloudInitializer, controllerInitializers(), names.CCMControllerAliases(), flagSet, wait.NeverStop)
	command.Use = "thalassa-cloud-controller-manager"

	code := cli.Run(command)
	os.Exit(code)
}

func controllerInitializers() map[string]app.ControllerInitFuncConstructor {
	controllerInitializers := app.DefaultInitFuncConstructors
	if constructor, ok := controllerInitializers[names.CloudNodeController]; ok {
		constructor.InitContext.ClientName = "thalassa-external-cloud-node-controller"
		controllerInitializers[names.CloudNodeController] = constructor
	}
	if constructor, ok := controllerInitializers[names.CloudNodeLifecycleController]; ok {
		constructor.InitContext.ClientName = "thalassa-external-cloud-node-lifecycle-controller"
		controllerInitializers[names.CloudNodeLifecycleController] = constructor
	}
	if constructor, ok := controllerInitializers[names.ServiceLBController]; ok {
		constructor.InitContext.ClientName = "thalassa-external-service-controller"
		controllerInitializers[names.ServiceLBController] = constructor
	}
	return controllerInitializers
}

func cloudInitializer(config *config.CompletedConfig) cloudprovider.Interface {
	cloudConfig := config.ComponentConfig.KubeCloudShared.CloudProvider

	// initialize cloud provider with the cloud provider name and config file provided
	cloud, err := cloudprovider.InitCloudProvider(cloudConfig.Name, cloudConfig.CloudConfigFile)
	if err != nil {
		klog.Fatalf("cloud provider could not be initialized: %v", err)
	}
	if cloud == nil {
		klog.Fatalf("cloud provider is nil")
	}

	if !cloud.HasClusterID() {
		if config.ComponentConfig.KubeCloudShared.AllowUntaggedCloud {
			klog.Fatalf("no ClusterID found. A ClusterID is required for the cloud provider to function properly")
		} else {
			klog.Fatalf("no ClusterID found. A ClusterID is required for the cloud provider to function properly")
		}
	}
	return cloud
}

func init() {
	versionpkg.Init(version, commit, date, builtBy)
}
