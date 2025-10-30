# Cloud Provider Thalassa

This repository holds the code for the Cloud Controller Manager (CCM) for Thalassa Cloud. The CCM is a Kubernetes control plane component that embeds cloud-specific control logic, allowing Kubernetes to interact with Thalassa Cloud's infrastructure services.

## Overview

The Cloud Controller Manager integrates Kubernetes with Thalassa Cloud's infrastructure services.

## Features

- Load balancers for Services of type `LoadBalancer` are created and kept in sync
- Per-port access control lists (ACLs) via annotations; global and per-port ACLs can be combined
- Optional managed security group per Service (created, updated and cleaned up automatically)
- Node metadata and lifecycle integration
- Zone and region labels for nodes

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


