# Load Balancer Annotations

The Thalassa Cloud Provider supports various annotations on Kubernetes Services to configure load balancer behavior. All annotations use the prefix `loadbalancer.k8s.thalassa.cloud/`.

## Annotations

| Annotation                                                       | Type                   | Default             | Description                                                            |
| ---------------------------------------------------------------- | ---------------------- | ------------------- | ---------------------------------------------------------------------- |
| `loadbalancer.k8s.thalassa.cloud/subnet`                         | String                 | First subnet in VPC | Subnet ID where the load balancer should be deployed                   |
| `loadbalancer.k8s.thalassa.cloud/type`                           | String                 | `"public"`          | Type of load balancer to create                                        |
| `loadbalancer.k8s.thalassa.cloud/internal`                       | Boolean                | `false`             | Create an internal load balancer (immutable after creation)            |
| `loadbalancer.k8s.thalassa.cloud/security-groups`                | Comma-separated string | Empty               | Security group IDs to attach to the load balancer                      |
| `loadbalancer.k8s.thalassa.cloud/create-security-group`          | Boolean                | `false`             | Automatically create and manage a security group for the load balancer |
| `loadbalancer.k8s.thalassa.cloud/acl-allowed-sources`            | Comma-separated string | Empty (allow all)   | Global CIDR ranges allowed to access all listener ports                |
| `loadbalancer.k8s.thalassa.cloud/acl-port-{port-name-or-number}` | Comma-separated string | Empty               | Per-port CIDR ranges (combined with global ACL)                        |
| `loadbalancer.k8s.thalassa.cloud/loadbalancing-policy`           | String                 | `"ROUND_ROBIN"`     | Load balancing algorithm (ROUND_ROBIN, RANDOM, MAGLEV)                 |
| `loadbalancer.k8s.thalassa.cloud/health-check-enabled`           | Boolean                | `false`             | Enable health checks for the target group                              |
| `loadbalancer.k8s.thalassa.cloud/health-check-port`              | Integer                | Required if enabled | Port for health checks (1-65535)                                       |
| `loadbalancer.k8s.thalassa.cloud/health-check-path`              | String                 | `"/healthz"`        | HTTP path for health checks                                            |
| `loadbalancer.k8s.thalassa.cloud/health-check-protocol`          | String                 | `"http"`            | Protocol for health checks (http, tcp)                                 |
| `loadbalancer.k8s.thalassa.cloud/health-check-interval`          | Integer (seconds)      | `10`                | Time interval between health checks                                    |
| `loadbalancer.k8s.thalassa.cloud/health-check-timeout`           | Integer (seconds)      | `5`                 | Maximum time to wait for health check response                         |
| `loadbalancer.k8s.thalassa.cloud/health-check-up-threshold`      | Integer                | `2`                 | Consecutive successful checks before backend is healthy                |
| `loadbalancer.k8s.thalassa.cloud/health-check-down-threshold`    | Integer                | `3`                 | Consecutive failed checks before backend is unhealthy                  |
| `loadbalancer.k8s.thalassa.cloud/idle-connection-timeout`        | Integer (seconds)      | `6000`              | Maximum idle time before closing connection                            |
| `loadbalancer.k8s.thalassa.cloud/max-connections`                | Integer                | `10000`             | Maximum concurrent connections allowed                                 |
| `loadbalancer.k8s.thalassa.cloud/enable-proxy-protocol`          | Boolean                | `false`             | Enable PROXY protocol (v1) for preserving client IP                    |

## Basic Configuration

### Subnet Selection

**Annotation:** `loadbalancer.k8s.thalassa.cloud/subnet`

**Type:** String

**Default:** First subnet in the VPC

**Description:** Specifies the subnet ID where the load balancer should be deployed. If not specified, the first available subnet in the VPC will be used.

**Example:**

```yaml
apiVersion: v1
kind: Service
metadata:
  name: my-service
  annotations:
    loadbalancer.k8s.thalassa.cloud/subnet: "subnet-12345678"
spec:
  type: LoadBalancer
  # ... rest of service spec
```

### Load Balancer Type

**Annotation:** `loadbalancer.k8s.thalassa.cloud/type`

**Type:** String

**Default:** `"public"`

**Description:** Specifies the type of load balancer to create. Currently supports `"public"`.

**Example:**

```yaml
apiVersion: v1
kind: Service
metadata:
  name: my-service
  annotations:
    loadbalancer.k8s.thalassa.cloud/type: "public"
spec:
  type: LoadBalancer
```

### Internal Load Balancer

**Annotation:** `loadbalancer.k8s.thalassa.cloud/internal`

**Type:** Boolean (`"true"` or `"false"`)

**Default:** `false`

**Description:** When set to `true`, creates an internal load balancer that is not accessible from the internet. Can only be set during load balancer creation and cannot be changed afterward.

**Example:**

```yaml
apiVersion: v1
kind: Service
metadata:
  name: my-service
  annotations:
    loadbalancer.k8s.thalassa.cloud/internal: "true"
spec:
  type: LoadBalancer
```

## Network Configuration

### Security Groups

**Annotation:** `loadbalancer.k8s.thalassa.cloud/security-groups`

**Type:** Comma-separated string

**Default:** Empty (no security groups)

**Description:** Comma-separated list of security group IDs to attach to the load balancer. All security groups must exist in the same VPC as the load balancer.

**Example:**

```yaml
apiVersion: v1
kind: Service
metadata:
  name: my-service
  annotations:
    loadbalancer.k8s.thalassa.cloud/security-groups: "sg-12345678,sg-87654321"
spec:
  type: LoadBalancer
```

### Access Control Lists (ACL)

#### Global ACL Configuration

**Annotation:** `loadbalancer.k8s.thalassa.cloud/acl-allowed-sources`

**Type:** Comma-separated string

**Default:** Empty (allow all sources)

**Description:** Comma-separated list of CIDR ranges that are allowed to access all load balancer listener ports. Supports both IPv4 and IPv6 CIDR ranges, but must be compatible with the public network used (e.g., IPv4 CIDR ranges for IPv4 load balancers).

**Example:**

```yaml
apiVersion: v1
kind: Service
metadata:
  name: my-service
  annotations:
    loadbalancer.k8s.thalassa.cloud/acl-allowed-sources: "10.0.0.0/8,192.168.1.0/24"
spec:
  type: LoadBalancer
```

#### Per-Port ACL Configuration

**Annotation:** `loadbalancer.k8s.thalassa.cloud/acl-port-{port-name-or-number}`

**Type:** Comma-separated string

**Default:** Empty (no per-port restrictions)

**Description:** Allows you to configure different ACL rules for specific ports. You can use either the port name or port number in the annotation key. When both global and per-port ACLs are configured, they are combined (union) for each port.

**Supported Formats:**

- By port name: `loadbalancer.k8s.thalassa.cloud/acl-port-http`
- By port number: `loadbalancer.k8s.thalassa.cloud/acl-port-80`

**Example with port names:**

```yaml
apiVersion: v1
kind: Service
metadata:
  name: my-service
  annotations:
    loadbalancer.k8s.thalassa.cloud/acl-port-http: "10.0.0.0/8"
    loadbalancer.k8s.thalassa.cloud/acl-port-https: "172.16.0.0/12"
spec:
  type: LoadBalancer
  ports:
    - name: http
      port: 80
      targetPort: 8080
    - name: https
      port: 443
      targetPort: 8443
```

**Example with port numbers:**

```yaml
apiVersion: v1
kind: Service
metadata:
  name: my-service
  annotations:
    loadbalancer.k8s.thalassa.cloud/acl-port-80: "10.0.0.0/8"
    loadbalancer.k8s.thalassa.cloud/acl-port-443: "172.16.0.0/12"
spec:
  type: LoadBalancer
  ports:
    - port: 80
      targetPort: 8080
    - port: 443
      targetPort: 8443
```

**Example with combined global and per-port ACLs:**

```yaml
apiVersion: v1
kind: Service
metadata:
  name: my-service
  annotations:
    # Global ACL applies to all ports
    loadbalancer.k8s.thalassa.cloud/acl-allowed-sources: "10.0.0.0/8,192.168.1.0/24"
    # Per-port ACLs are combined with global ACL
    loadbalancer.k8s.thalassa.cloud/acl-port-http: "172.16.0.0/12"
    loadbalancer.k8s.thalassa.cloud/acl-port-443: "10.10.0.0/16"
spec:
  type: LoadBalancer
  ports:
    - name: http
      port: 80
      targetPort: 8080
    - name: https
      port: 443
      targetPort: 8443
```

**Result:**

- Port 80 (http): `10.0.0.0/8`, `192.168.1.0/24`, `172.16.0.0/12`
- Port 443 (https): `10.0.0.0/8`, `192.168.1.0/24`, `10.10.0.0/16`

## Load Balancing Policy

### Load Balancing Algorithm

**Annotation:** `loadbalancer.k8s.thalassa.cloud/loadbalancing-policy`

**Type:** String

**Default:** `"ROUND_ROBIN"`

**Valid Values:**

- `"ROUND_ROBIN"`: Connections are distributed across all target group attachments in a round-robin fashion
- `"RANDOM"`: Connections are distributed across all target group attachments randomly
- `"MAGLEV"`: Connections are distributed using the Maglev consistent hashing algorithm

**Description:** Specifies the load balancing algorithm to use for distributing traffic across backend targets.

**Example:**

```yaml
apiVersion: v1
kind: Service
metadata:
  name: my-service
  annotations:
    loadbalancer.k8s.thalassa.cloud/loadbalancing-policy: "MAGLEV"
spec:
  type: LoadBalancer
```

## Health Checks

### Enable Health Checks

**Annotation:** `loadbalancer.k8s.thalassa.cloud/health-check-enabled`

**Type:** Boolean (`"true"` or `"false"`)

**Default:** `false`

**Description:** Enables health checks for the load balancer target group. When enabled, the load balancer will periodically check the health of backend targets.

**Example:**

```yaml
apiVersion: v1
kind: Service
metadata:
  name: my-service
  annotations:
    loadbalancer.k8s.thalassa.cloud/health-check-enabled: "true"
    loadbalancer.k8s.thalassa.cloud/health-check-port: "8080"
spec:
  type: LoadBalancer
```

### Health Check Port

**Annotation:** `loadbalancer.k8s.thalassa.cloud/health-check-port`

**Type:** Integer

**Default:** Not set (required when health checks are enabled)

**Range:** 1-65535

**Description:** The port to use for health checks. **Required** when health checks are enabled.

**Example:**

```yaml
apiVersion: v1
kind: Service
metadata:
  name: my-service
  annotations:
    loadbalancer.k8s.thalassa.cloud/health-check-enabled: "true"
    loadbalancer.k8s.thalassa.cloud/health-check-port: "8080"
spec:
  type: LoadBalancer
```

### Health Check Path

**Annotation:** `loadbalancer.k8s.thalassa.cloud/health-check-path`

**Type:** String

**Default:** `"/healthz"`

**Description:** The HTTP path to use for health checks when using HTTP protocol.

**Example:**

```yaml
apiVersion: v1
kind: Service
metadata:
  name: my-service
  annotations:
    loadbalancer.k8s.thalassa.cloud/health-check-enabled: "true"
    loadbalancer.k8s.thalassa.cloud/health-check-port: "8080"
    loadbalancer.k8s.thalassa.cloud/health-check-path: "/health"
spec:
  type: LoadBalancer
```

### Health Check Protocol

**Annotation:** `loadbalancer.k8s.thalassa.cloud/health-check-protocol`

**Type:** String

**Default:** `"http"`

**Valid Values:** `"http"`, `"tcp"`

**Description:** The protocol to use for health checks.

**Example:**

```yaml
apiVersion: v1
kind: Service
metadata:
  name: my-service
  annotations:
    loadbalancer.k8s.thalassa.cloud/health-check-enabled: "true"
    loadbalancer.k8s.thalassa.cloud/health-check-port: "8080"
    loadbalancer.k8s.thalassa.cloud/health-check-protocol: "tcp"
spec:
  type: LoadBalancer
```

### Health Check Interval

**Annotation:** `loadbalancer.k8s.thalassa.cloud/health-check-interval`

**Type:** Integer (seconds)

**Default:** `10`

**Description:** The time interval between health checks in seconds.

**Example:**

```yaml
apiVersion: v1
kind: Service
metadata:
  name: my-service
  annotations:
    loadbalancer.k8s.thalassa.cloud/health-check-enabled: "true"
    loadbalancer.k8s.thalassa.cloud/health-check-port: "8080"
    loadbalancer.k8s.thalassa.cloud/health-check-interval: "30"
spec:
  type: LoadBalancer
```

### Health Check Timeout

**Annotation:** `loadbalancer.k8s.thalassa.cloud/health-check-timeout`

**Type:** Integer (seconds)

**Default:** `5`

**Description:** The maximum time to wait for a health check response in seconds.

**Example:**

```yaml
apiVersion: v1
kind: Service
metadata:
  name: my-service
  annotations:
    loadbalancer.k8s.thalassa.cloud/health-check-enabled: "true"
    loadbalancer.k8s.thalassa.cloud/health-check-port: "8080"
    loadbalancer.k8s.thalassa.cloud/health-check-timeout: "10"
spec:
  type: LoadBalancer
```

### Health Check Up Threshold

**Annotation:** `loadbalancer.k8s.thalassa.cloud/health-check-up-threshold`

**Type:** Integer

**Default:** `2`

**Description:** The number of consecutive successful health checks required before a backend is considered healthy.

**Example:**

```yaml
apiVersion: v1
kind: Service
metadata:
  name: my-service
  annotations:
    loadbalancer.k8s.thalassa.cloud/health-check-enabled: "true"
    loadbalancer.k8s.thalassa.cloud/health-check-port: "8080"
    loadbalancer.k8s.thalassa.cloud/health-check-up-threshold: "3"
spec:
  type: LoadBalancer
```

### Health Check Down Threshold

**Annotation:** `loadbalancer.k8s.thalassa.cloud/health-check-down-threshold`

**Type:** Integer

**Default:** `3`

**Description:** The number of consecutive failed health checks required before a backend is considered unhealthy.

**Example:**

```yaml
apiVersion: v1
kind: Service
metadata:
  name: my-service
  annotations:
    loadbalancer.k8s.thalassa.cloud/health-check-enabled: "true"
    loadbalancer.k8s.thalassa.cloud/health-check-port: "8080"
    loadbalancer.k8s.thalassa.cloud/health-check-down-threshold: "5"
spec:
  type: LoadBalancer
```

## Connection Settings

### Idle Connection Timeout

**Annotation:** `loadbalancer.k8s.thalassa.cloud/idle-connection-timeout`

**Type:** Integer (seconds)

**Default:** `6000`

**Description:** The maximum time in seconds to wait for a connection to be idle before closing it.

**Example:**

```yaml
apiVersion: v1
kind: Service
metadata:
  name: my-service
  annotations:
    loadbalancer.k8s.thalassa.cloud/idle-connection-timeout: "3000"
spec:
  type: LoadBalancer
```

### Maximum Connections

**Annotation:** `loadbalancer.k8s.thalassa.cloud/max-connections`

**Type:** Integer

**Default:** `10000`

**Description:** The maximum number of concurrent connections allowed to the load balancer.

**Example:**

```yaml
apiVersion: v1
kind: Service
metadata:
  name: my-service
  annotations:
    loadbalancer.k8s.thalassa.cloud/max-connections: "5000"
spec:
  type: LoadBalancer
```

### Enable Proxy Protocol

**Annotation:** `loadbalancer.k8s.thalassa.cloud/enable-proxy-protocol`

**Type:** Boolean (`"true"` or `"false"`)

**Default:** `false`

**Description:** Enables the PROXY protocol (v1) for preserving client IP addresses. When enabled, the load balancer will prepend a PROXY protocol header to incoming connections.

**Example:**

```yaml
apiVersion: v1
kind: Service
metadata:
  name: my-service
  annotations:
    loadbalancer.k8s.thalassa.cloud/enable-proxy-protocol: "true"
spec:
  type: LoadBalancer
```

## Examples

### Basic Load Balancer

```yaml
apiVersion: v1
kind: Service
metadata:
  name: web-app
  annotations:
    loadbalancer.k8s.thalassa.cloud/loadbalancing-policy: "ROUND_ROBIN"
spec:
  type: LoadBalancer
  ports:
    - port: 80
      targetPort: 8080
      protocol: TCP
  selector:
    app: web-app
```

### Load Balancer with Health Checks

```yaml
apiVersion: v1
kind: Service
metadata:
  name: api-service
  annotations:
    loadbalancer.k8s.thalassa.cloud/health-check-enabled: "true"
    loadbalancer.k8s.thalassa.cloud/health-check-port: "8080"
    loadbalancer.k8s.thalassa.cloud/health-check-path: "/health"
    loadbalancer.k8s.thalassa.cloud/health-check-interval: "30"
    loadbalancer.k8s.thalassa.cloud/health-check-timeout: "10"
    loadbalancer.k8s.thalassa.cloud/health-check-up-threshold: "3"
    loadbalancer.k8s.thalassa.cloud/health-check-down-threshold: "5"
    loadbalancer.k8s.thalassa.cloud/loadbalancing-policy: "MAGLEV"
spec:
  type: LoadBalancer
  ports:
    - port: 443
      targetPort: 8443
      protocol: TCP
  selector:
    app: api-service
```

### Internal Load Balancer with Security Groups

```yaml
apiVersion: v1
kind: Service
metadata:
  name: internal-service
  annotations:
    loadbalancer.k8s.thalassa.cloud/internal: "true"
    loadbalancer.k8s.thalassa.cloud/security-groups: "sg-12345678,sg-87654321"
    loadbalancer.k8s.thalassa.cloud/subnet: "subnet-abcdef12"
spec:
  type: LoadBalancer
  ports:
    - port: 3306
      targetPort: 3306
      protocol: TCP
  selector:
    app: database
```

### Load Balancer with Access Control

```yaml
apiVersion: v1
kind: Service
metadata:
  name: restricted-service
  annotations:
    loadbalancer.k8s.thalassa.cloud/acl-allowed-sources: "10.0.0.0/8,172.16.0.0/12"
    loadbalancer.k8s.thalassa.cloud/max-connections: "1000"
    loadbalancer.k8s.thalassa.cloud/idle-connection-timeout: "3000"
    loadbalancer.k8s.thalassa.cloud/enable-proxy-protocol: "true"
spec:
  type: LoadBalancer
  ports:
    - port: 80
      targetPort: 8080
      protocol: TCP
  selector:
    app: restricted-app
```

### Load Balancer with Per-Port ACL Configuration

```yaml
apiVersion: v1
kind: Service
metadata:
  name: multi-port-service
  annotations:
    # Global ACL for all ports
    loadbalancer.k8s.thalassa.cloud/acl-allowed-sources: "10.0.0.0/8"
    # Per-port ACLs for specific ports
    loadbalancer.k8s.thalassa.cloud/acl-port-http: "192.168.1.0/24"
    loadbalancer.k8s.thalassa.cloud/acl-port-443: "172.16.0.0/12"
    loadbalancer.k8s.thalassa.cloud/acl-port-8080: "10.10.0.0/16"
    # Health checks for API port
    loadbalancer.k8s.thalassa.cloud/health-check-enabled: "true"
    loadbalancer.k8s.thalassa.cloud/health-check-port: "8080"
    loadbalancer.k8s.thalassa.cloud/health-check-path: "/health"
spec:
  type: LoadBalancer
  ports:
    - name: http
      port: 80
      targetPort: 8080
      protocol: TCP
    - name: https
      port: 443
      targetPort: 8443
      protocol: TCP
    - name: api
      port: 8080
      targetPort: 8080
      protocol: TCP
  selector:
    app: multi-port-app
```

**Resulting ACL Configuration:**

- Port 80 (http): `10.0.0.0/8`, `192.168.1.0/24`
- Port 443 (https): `10.0.0.0/8`, `172.16.0.0/12`
- Port 8080 (api): `10.0.0.0/8`, `10.10.0.0/16`

## Notes

1. **Immutable Settings**: Some annotations like `loadbalancer.k8s.thalassa.cloud/internal` can only be set during load balancer creation and cannot be changed afterward.

2. **Required Combinations**: When health checks are enabled, the `loadbalancer.k8s.thalassa.cloud/health-check-port` annotation is required.

3. **Validation**: Invalid annotation values will cause the load balancer creation or update to fail. Check the cloud provider logs for validation errors.

4. **External Traffic Policy**: The cloud provider automatically filters nodes based on the service's `externalTrafficPolicy` setting:
   - `Local`: Only nodes with ready endpoints are included in the load balancer
   - `Cluster`: All nodes are included

5. **Node Filtering**: The cloud provider automatically resyncs load balancers when pods move between nodes for services with `externalTrafficPolicy: Local`.

6. **Per-Port ACL Configuration**: You can configure different ACL rules for different ports using the `loadbalancer.k8s.thalassa.cloud/acl-port-{port-name-or-number}` annotation format. Both port names and port numbers are supported. When both global and per-port ACLs are configured, they are combined (union) for each port.

7. **ACL Annotation Priority**: Per-port ACL annotations take precedence over global ACL annotations. If a port has both a port name and port number annotation, both are combined. Invalid CIDR ranges in annotations are logged as errors and skipped.
