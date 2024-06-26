
# Envoy ext_authz Plugin with Permit.io

This repository is a proof of concept for creating an `ext_authz` plugin for Envoy to use with [Permit.io](https://permit.io/). The plugin is implemented in Go and uses gRPC to communicate with Envoy.

## Overview

The authorization server intercepts incoming HTTP requests to Envoy and checks the provided authorization header against Permit.io policies. It verifies the JWT token, extracts the claims, and performs authorization checks based on the request method and custom resource attributes.

## Prerequisites

- Go 1.16 or later
- Envoy Proxy
- Permit.io account
- PDP Key and PDP URL from Permit.io

## Setup

1. Clone the repository:
   ```sh
   git clone https://github.com/yourusername/envoy-ext-authz-permitio.git
   cd envoy-ext-authz-permitio
   ```

2. Install the Go dependencies:
   ```sh
   go mod tidy
   ```

3. Set up your environment variables with the Permit.io PDP Key and PDP URL:
   ```sh
   export PDP_KEY="your_pdp_key"
   export PDP_URL="your_pdp_url"
   ```

4. Run the authorization server:
   ```sh
   go run main.go
   ```

The server will start listening on port `9192`.

## Envoy Configuration

To configure Envoy to use this external authorization server, add the following to your Envoy configuration file:

```yaml
static_resources:
  listeners:
  - name: listener_0
    address:
      socket_address:
        address: 0.0.0.0
        port_value: 10000
    filter_chains:
    - filters:
      - name: envoy.filters.network.http_connection_manager
        typed_config:
          "@type": type.googleapis.com/envoy.extensions.filters.network.http_connection_manager.v3.HttpConnectionManager
          stat_prefix: ingress_http
          route_config:
            name: local_route
            virtual_hosts:
            - name: local_service
              domains: ["*"]
              routes:
              - match: { prefix: "/" }
                route: { cluster: some_service }
          http_filters:
          - name: envoy.filters.http.ext_authz
            typed_config:
              "@type": type.googleapis.com/envoy.extensions.filters.http.ext_authz.v3.ExtAuthz
              grpc_service:
                envoy_grpc:
                  cluster_name: ext_authz
                timeout: 0.25s
          - name: envoy.filters.http.router

  clusters:
  - name: some_service
    connect_timeout: 0.25s
    type: LOGICAL_DNS
    lb_policy: ROUND_ROBIN
    load_assignment:
      cluster_name: some_service
      endpoints:
      - lb_endpoints:
        - endpoint:
            address:
              socket_address:
                address: some_service_address
                port_value: some_service_port

  - name: ext_authz
    connect_timeout: 0.25s
    type: LOGICAL_DNS
    lb_policy: ROUND_ROBIN
    load_assignment:
      cluster_name: ext_authz
      endpoints:
      - lb_endpoints:
        - endpoint:
            address:
              socket_address:
                address: 127.0.0.1
                port_value: 9192
```

## How It Works

1. The Envoy proxy receives an incoming HTTP request and forwards it to the external authorization server.
2. The authorization server checks the `authorization` header for a Bearer token.
3. The JWT token is parsed and validated.
4. The server extracts the necessary claims from the token and constructs a user and resource object.
5. It checks the permission using Permit.io's `Check` method.
6. Based on the result, it responds to Envoy with either an `OK` or `Denied` response.

## License

This project is licensed under the MIT License. See the [LICENSE](LICENSE) file for details.

## Contributing

Contributions are welcome! Please open an issue or submit a pull request.