# Policy Wrapping Prototype

## Overview

This is a prototype to show the usage of [kustomize](https://kustomize.io/) for wrapping policies.

## To Do

* Add placement rule and placement binding support

## Examples

### With a Kustomize Patch

The following command uses a Kustomize patch that is missing the identifiers of name, namespace,
apiVersion, and kind. Those are automatically filled in by the program.

```bash
go run main.go -namespace my-policies -name policy-app-config -patches input/patch.yaml input/configmap.yaml
```

Output:

```yaml
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

### Use Defaults

The following command relies on the defaults provided by the program:

```bash
go run main.go -namespace my-policies -name policy-app-config input/configmap.yaml
```

Output:

```yaml
apiVersion: policy.open-cluster-management.io/v1
kind: Policy
metadata:
  annotations:
    policy.open-cluster-management.io/categories: CM Configuration Management
    policy.open-cluster-management.io/controls: CM-2 Baseline Configuration
    policy.open-cluster-management.io/standards: NIST SP 800-53
  name: policy-app-config
  namespace: my-policies
spec:
  disabled: true
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
  remediationAction: inform
```

### Some Overrides

The following command relies on defaults but provides a couple overrides using flags:

```bash
go run main.go -namespace my-policies -name policy-app-config -disabled=false -remediationAction=enforce input/configmap.yaml
```

Output:

```yaml
apiVersion: policy.open-cluster-management.io/v1
kind: Policy
metadata:
  annotations:
    policy.open-cluster-management.io/categories: CM Configuration Management
    policy.open-cluster-management.io/controls: CM-2 Baseline Configuration
    policy.open-cluster-management.io/standards: NIST SP 800-53
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
        remediationAction: enforce
        severity: low
  remediationAction: enforce
```
