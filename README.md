# Policy Wrapping Prototype

## Overview

This is a prototype to show the usage of [kustomize](https://kustomize.io/) for wrapping policies.

## Examples

### With a Kustomize Patch

The following commands must be run for setup:

```bash
# Compile the plugin in a temporary directory under $DEMO
export DEMO=$(mktemp -d)
export PLUGIN_ROOT_PARENT_DIR=$DEMO/kustomize/plugin/policygenerator.open-cluster-management.io/v1
mkdir -p $PLUGIN_ROOT_PARENT_DIR
export PLUGIN_ROOT=$PLUGIN_ROOT_PARENT_DIR/policygenerator
go build -buildmode plugin -o $PLUGIN_ROOT/PolicyGenerator.so PolicyGenerator.go

# Compile kustomize (required for Go based plugins)
GO111MODULE=on go get sigs.k8s.io/kustomize/kustomize/v4
```

The following command will utilize the `kustomization.yaml` in the root of the repository:

```bash
XDG_CONFIG_HOME=$DEMO $(go env GOPATH)/bin/kustomize build --enable-alpha-plugins
```

Output:

```yaml
apiVersion: policy.open-cluster-management.io/v1
kind: PlacementBinding
metadata:
  labels:
    custom: myApp
  name: my-placement-binding
  namespace: my-policies
placementRef:
  apiGroup: apps.open-cluster-management.io/v1
  kind: PlacementRule
  name: placement-red-hat-cloud
subjects:
- apiGroup: policy.open-cluster-management.io/v1
  kind: Policy
  name: policy-app-config
- apiGroup: policy.open-cluster-management.io/v1
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
