# Policy Wrapping Prototype

## Overview

This is a prototype to show the usage of [kustomize](https://kustomize.io/) for wrapping policies.

## Examples

### With a Kustomize Patch

The following commands must be run for setup:

```bash
# Compile the plugin in a temporary directory under $DEMO
DEMO=$(mktemp -d)
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
apiVersion: apps.open-cluster-management.io/v1
kind: PlacementRule
metadata:
  name: placement-policy-app-config
  namespace: my-policies
spec:
  clusterConditions:
  - status: "True"
    type: ManagedClusterConditionAvailable
  clusterSelector:
    matchExpressions:
    - key: cloud
      operator: In
      values:
      - red hat
---
apiVersion: policy.open-cluster-management.io/v1
kind: PlacementBinding
metadata:
  name: binding-policy-app-config
  namespace: my-policies
placementRef:
  apiGroup: apps.open-cluster-management.io/v1
  kind: PlacementRule
  name: placement-policy-app-config
subjects:
- apiGroup: policy.open-cluster-management.io/v1
  kind: Policy
  name: policy-app-config
---
apiVersion: policy.open-cluster-management.io/v1
kind: Policy
metadata:
  annotations:
    policy.open-cluster-management.io/categories: PR.DS Data Security
    policy.open-cluster-management.io/controls: CM-2 Baseline Configuration
    policy.open-cluster-management.io/standards: NIST SP 800-53
  labels:
    app: super-secure-app
  name: policy-app-config
  namespace: my-policies
spec:
  disabled: false
  policy-templates:
  - objectDefinition:
      apiVersion: policy.open-cluster-management.io/v1
      kind: ConfigurationPolicy
      spec:
        object-templates:
        - apiVersion: v1
          data:
            game.properties: "enemies=aliens\nlives=3\nenemies.cheat=true\nenemies.cheat.level=noGoodRotten\nsecret.code.passphrase=UUDDLRLRBABAS\nsecret.code.allowed=true\nsecret.code.lives=30
              \   \n"
            ui.properties: "color.good=purple\ncolor.bad=yellow\nallow.textmode=true\nhow.nice.to.look=fairlyNice
              \n"
          kind: ConfigMap
          metadata:
            name: game-config
            namespace: default
        remediationAction: inform
        severity: low
  remediationAction: enforce
```
