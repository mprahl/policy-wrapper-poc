# Policy Wrapping Prototype

## Overview

This is a prototype to show the usage of [kustomize](https://kustomize.io/) for wrapping policies.

## Using the Policy Generator

### As a Kustomize plugin

1. Build the plugin binary (only needed once or to update the plugin):
    ```bash
    make build
    ```
    **NOTE:** This will default to placing the binary in `${HOME}/.config/kustomize/plugin/`. You can change this by exporting `KUSTOMIZE_PLUGIN_HOME` to a different path.

2. Create a `kustomization.yaml` file that points to `PolicyGenerator` manifest(s), with any additional desired patches or customizations (see [`examples/policyGenerator.yaml`](./examples/policyGenerator.yaml) for an example):
    ```yaml
    generators:
    - path/to/generator/file.yaml
    ```

3. To use the plugin to generate policies, do one of:
    - Utilize the `kustomization.yaml` in the `examples/` directory of the repository (the directory can be modified by exporting a new path to `SOURCE_DIR`):
      ```bash
      make generate
      ```
    - From any directory with a `kustomization.yaml` file pointing to `PolicyGenerator` manifests:
      ```bash
      kustomize build --enable-alpha-plugins
      ```

**Output from `examples/`:**
```yaml
apiVersion: policy.open-cluster-management.io/v1
kind: PlacementBinding
metadata:
  labels:
    custom: myApp
  name: my-placement-binding
  namespace: my-policies
placementRef:
  apiGroup: apps.open-cluster-management.io
  kind: PlacementRule
  name: placement-red-hat-cloud
subjects:
- apiGroup: policy.open-cluster-management.io
  kind: Policy
  name: policy-app-config
- apiGroup: policy.open-cluster-management.io
  kind: Policy
  name: policy-app-config2
---
apiVersion: policy.open-cluster-management.io/v1
kind: Policy
metadata:
  annotations:
    policy.open-cluster-management.io/categories: PR.DS Data Security
    policy.open-cluster-management.io/controls: PR.DS-1 Data-at-rest
    policy.open-cluster-management.io/standards: NIST SP 800-53
  labels:
    app: super-secure-app
    custom: myApp
  name: policy-app-config
  namespace: my-policies
spec:
  disabled: false
  policy-templates:
  - objectDefinition:
      apiVersion: policy.open-cluster-management.io/v1
      kind: ConfigurationPolicy
      name: policy-app-config
      spec:
        object-templates:
        - apiVersion: v1
          data:
            game.properties: |
              enemies=aliens
            ui.properties: |
              color.good=purple
          kind: ConfigMap
          metadata:
            name: game-config
            namespace: default
        remediationAction: enforce
        severity: medium
  remediationAction: enforce
---
apiVersion: policy.open-cluster-management.io/v1
kind: Policy
metadata:
  annotations:
    policy.open-cluster-management.io/categories: CM Configuration Management
    policy.open-cluster-management.io/controls: PR.DS-1 Data-at-rest
    policy.open-cluster-management.io/standards: NIST SP 800-53
  labels:
    custom: myApp
  name: policy-app-config2
  namespace: my-policies
spec:
  disabled: true
  policy-templates:
  - objectDefinition:
      apiVersion: policy.open-cluster-management.io/v1
      kind: ConfigurationPolicy
      name: policy-app-config2
      spec:
        object-templates:
        - apiVersion: v1
          data:
            game.properties: "enemies=goldfish  \n"
            ui.properties: |
              color.good=neon-green
          kind: ConfigMap
          metadata:
            name: game-config2
            namespace: default
        - apiVersion: v1
          data:
            game.properties: "enemies=toads  \n"
            ui.properties: |
              color.good=cherry-red
          kind: ConfigMap
          metadata:
            name: game-config3
            namespace: default
        remediationAction: inform
        severity: medium
  remediationAction: inform
```

### As a standalone binary

In order to bypass Kustomize and run the generator binary directly:

1. Build the binary:
    ```bash
    make build-binary
    ```

2. Run the binary from the location of the PolicyGenerator manifest(s):
    ```bash
    path/to/PolicyGenerator <path/to/file/1> ... <path/to/file/n>
    ```
    - For example:
      ```bash
      cd examples
      ../PolicyGenerator policyGenerator.yaml
      ```
    **NOTE:** To print the trace, you can add the `--debug` flag to the arguments.