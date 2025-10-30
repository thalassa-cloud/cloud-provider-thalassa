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
	// LoadbalancerAnnotationIdleConnectionTimeout is the maximum time in seconds to wait for a connection to be idle. Default is 6000.
	LoadbalancerAnnotationIdleConnectionTimeout = "loadbalancer.k8s.thalassa.cloud/idle-connection-timeout"
	// LoadbalancerAnnotationMaxConnections is the maximum number of connections to the loadbalancer. Default is 10000.
	LoadbalancerAnnotationMaxConnections = "loadbalancer.k8s.thalassa.cloud/max-connections"

	// LoadbalancerAnnotationLoadbalancingPolicy is the loadbalancing policy to use.
	// Must be one of ROUND_ROBIN, RANDOM, or MAGLEV.
	// The default policy is ROUND_ROBIN.
	// ROUND_ROBIN: Connections from a listener to the target group are distributed across all target group attachments.
	// RANDOM: Connections from a listener to the target group are distributed across all target group attachments in a random manner.
	// MAGLEV: Connections from a listener to the target group are distributed across all target group attachments based on the MAGLEV algorithm.
	LoadbalancerAnnotationLoadbalancingPolicy = "loadbalancer.k8s.thalassa.cloud/loadbalancing-policy"

	// LoadbalancerAnnotationHealthCheckEnabled is a boolean that indicates if health checks are enabled. Default is false.
	LoadbalancerAnnotationHealthCheckEnabled = "loadbalancer.k8s.thalassa.cloud/health-check-enabled"
	// LoadbalancerAnnotationHealthCheckPath is the health check path to use. Default is /healthz.
	LoadbalancerAnnotationHealthCheckPath = "loadbalancer.k8s.thalassa.cloud/health-check-path"
	// LoadbalancerAnnotationHealthCheckPort is the port to use for health checks. Must be between 1 and 65535 and must be provided if health check is enabled.
	LoadbalancerAnnotationHealthCheckPort = "loadbalancer.k8s.thalassa.cloud/health-check-port"
	// LoadbalancerAnnotationHealthCheckProtocol is the protocol to use for health checks.
	LoadbalancerAnnotationHealthCheckProtocol = "loadbalancer.k8s.thalassa.cloud/health-check-protocol"
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

	// LoadbalancerAnnotationAclAllowedSourcesPort is a per-port ACL configuration annotation.
	// Format: loadbalancer.k8s.thalassa.cloud/acl-port-{port-name-or-number}
	// Example: loadbalancer.k8s.thalassa.cloud/acl-port-http or loadbalancer.k8s.thalassa.cloud/acl-port-80
	// Value: comma separated list of CIDR ranges (same format as global acl-allowed-sources)
	// When both global and per-port ACLs are configured, they are combined
	LoadbalancerAnnotationAclAllowedSourcesPort = "loadbalancer.k8s.thalassa.cloud/acl-port"

	// LoadBalancerAnnotationSecurityGroups is a comma separated list of security group IDs to apply to the loadbalancer.
	LoadBalancerAnnotationSecurityGroups = "loadbalancer.k8s.thalassa.cloud/security-groups"
)

const (
	DefaultIdleConnectionTimeout = 6000
	DefaultMaxConnections        = 10000
	DefaultEnableProxyProtocol   = false
	DefaultLoadbalancingPolicy   = "ROUND_ROBIN"
)
