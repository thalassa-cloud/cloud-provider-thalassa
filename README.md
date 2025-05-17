# Cloud Provider Thalassa

This repository holds the code for the Cloud Controller Manager (CCM) for Thalassa Cloud. The CCM is a Kubernetes control plane component that embeds cloud-specific control logic, allowing Kubernetes to interact with Thalassa Cloud's infrastructure services.

## Overview

The Cloud Controller Manager integrates Kubernetes with Thalassa Cloud's infrastructure services.

## Features

- **Load Balancer Integration**: Automatically provisions and manages load balancers for Kubernetes services
- **Node Lifecycle Management**: Monitors node health and handles node lifecycle events
- **Instance Metadata**: Provides cloud-specific metadata for nodes
- **Zone and Region Support**: Supports availability zones and regions for proper node placement
- **Configurable Authentication**: Supports both Personal Access Token and OIDC authentication methods

## Configuration

The CCM is configured through a cloud configuration file:

```yaml
# Basic configuration
organisation: "your-org"
project: "your-project"
endpoint: "https://api.thalassa.cloud"
insecure: false

# Authentication (choose one)
cloudCredentials:
  personalAccessToken: "your-token"
  # OR
  clientID: "your-client-id"
  clientSecret: "your-client-secret"

# Infrastructure configuration
vpcIdentity: "your-vpc-id"
defaultSubnet: "your-subnet"
cluster: "your-cluster-id"

# Feature configuration
loadBalancer:
  enabled: true
  creationPollInterval: 5  # seconds
  creationPollTimeout: 300  # seconds

instancesV2:
  enabled: true
  zoneAndRegionEnabled: true

# Additional labels to be added to cloud resources
additionalLabels:
  key1: value1
  key2: value2
```

## Installation

1. Create a cloud configuration file with your settings
2. Deploy the CCM using the provided Kubernetes manifests
3. Configure your Kubernetes cluster to use the Thalassa cloud provider

## Development

### Prerequisites

- Go 1.21 or later
- Kubernetes 1.30 or later
- Access to Thalassa Cloud API

### Building

```bash
make build
```

### Testing

```bash
make test
```

## License

This project is licensed under the Apache License 2.0 


