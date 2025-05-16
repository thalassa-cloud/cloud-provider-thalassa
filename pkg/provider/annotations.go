package provider

const (
	// LoadBalancerAnnotationSubnetID is the ID of the subnet to use for the loadbalancer. Default is the first subnet in the VPC.
	LoadBalancerAnnotationSubnetID = "loadbalancer.k8s.thalassa.cloud/subnet"
	// LoadBalancerAnnotationLoadbalancerType is the type of loadbalancer to create. Default is "public".
	LoadBalancerAnnotationLoadbalancerType = "loadbalancer.k8s.thalassa.cloud/type"

	// LoadbalancerAnnotationInternal is a boolean that indicates if the loadbalancer should be internal. Default is false.
	// Can only be used upon loadbalancer creation.
	LoadbalancerAnnotationInternal = "loadbalancer.k8s.thalassa.cloud/internal"

	// LoadbalancerAnnotationEnableProxyProtocol is a boolean that enables the PROXY protocol. Default is false.
	LoadbalancerAnnotationEnableProxyProtocol = "loadbalancer.k8s.thalassa.cloud/enable-proxy-protocol"
	// LoadbalancerAnnotationEnableStickySessions is a boolean that enables sticky sessions. Default is false.
	LoadbalancerAnnotationEnableStickySessions = "loadbalancer.k8s.thalassa.cloud/enable-sticky-sessions"

	// LoadbalancerAnnotationServerTimeout is the maximum time in seconds to wait for a response from the server
	LoadbalancerAnnotationServerTimeout = "loadbalancer.k8s.thalassa.cloud/server-timeout"
	// LoadbalancerAnnotationClientTimeout is the maximum time in seconds to wait for a response from the server
	LoadbalancerAnnotationClientTimeout = "loadbalancer.k8s.thalassa.cloud/client-timeout"
	// LoadbalancerAnnotationConnectTimeout is the maximum time in seconds to wait for a connection to be established
	LoadbalancerAnnotationConnectTimeout = "loadbalancer.k8s.thalassa.cloud/connect-timeout"

	// LoadbalancerAnnotationHealthCheckPath is the health check path to use. Default is /healthz.
	LoadbalancerAnnotationHealthCheckPath = "loadbalancer.k8s.thalassa.cloud/health-check-path"
	// LoadbalancerAnnotationHealthCheckPort is the port to use for health checks. Default is 80.
	LoadbalancerAnnotationHealthCheckPort = "loadbalancer.k8s.thalassa.cloud/health-check-port"
	// LoadbalancerAnnotationHealthCheckInterval is the time in seconds between health checks
	LoadbalancerAnnotationHealthCheckInterval = "loadbalancer.k8s.thalassa.cloud/health-check-interval"
	// LoadbalancerAnnotationHealthCheckTimeout is the maximum time in seconds to wait for a health check response
	LoadbalancerAnnotationHealthCheckTimeout = "loadbalancer.k8s.thalassa.cloud/health-check-timeout"
	// LoadbalancerAnnotationHealthCheckUpThreshold is the number of consecutive successful health checks before a backend is considered up
	LoadbalancerAnnotationHealthCheckUpThreshold = "loadbalancer.k8s.thalassa.cloud/health-check-up-threshold"
	// LoadbalancerAnnotationHealthCheckDownThreshold is the number of consecutive failed health checks before a backend is considered down
	LoadbalancerAnnotationHealthCheckDownThreshold = "loadbalancer.k8s.thalassa.cloud/health-check-down-threshold"

	// LoadbalancerAnnotationAclAllowedSources is a comma separated list of CIDR ranges that are allowed to access the loadbalancer listener ports. Default no ACL, allow any source
	// CIDR ranges can be ipv4 or ipv6, but must be compatible with the public network used (i.g. ipv4 CIDR ranges for loadbalancers if the public network is ipv4)
	LoadbalancerAnnotationAclAllowedSources = "loadbalancer.k8s.thalassa.cloud/acl-allowed-sources"
)
