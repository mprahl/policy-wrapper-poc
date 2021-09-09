// Copyright Contributors to the Open Cluster Management project
package internal

import (
	"fmt"
	"io/ioutil"
	"path"
	"strings"
	"testing"
)

func TestGenerate(t *testing.T) {
	tmpDir := t.TempDir()
	createConfigMap(t, tmpDir, "configmap.yaml")
	p := Plugin{}
	p.PolicyDefaults.Namespace = "my-policies"
	policyConf := policyConfig{
		Name: "policy-app-config",
		Manifests: []manifest{
			{Path: path.Join(tmpDir, "configmap.yaml")},
		},
	}
	p.Policies = append(p.Policies, policyConf)
	p.applyDefaults()
	err := p.assertValidConfig()
	if err != nil {
		t.Fatal(err.Error())
	}

	expected := `
---
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
            metadata:
                name: policy-app-config
            spec:
                object-templates:
                    - complianceType: musthave
                      objectDefinition:
                        apiVersion: v1
                        data:
                            game.properties: enemies=potato
                        kind: ConfigMap
                        metadata:
                            name: my-configmap
                remediationAction: inform
                severity: low
    remediationAction: inform
---
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
        matchExpressions: []
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
`
	expected = strings.TrimPrefix(expected, "\n")
	output, err := p.Generate()
	if err != nil {
		t.Fatal(err.Error())
	}

	assertEqual(t, string(output), expected)
}

func TestCreatePolicy(t *testing.T) {
	tmpDir := t.TempDir()
	createConfigMap(t, tmpDir, "configmap.yaml")
	p := Plugin{}
	p.PolicyDefaults.Namespace = "my-policies"
	policyConf := policyConfig{
		Name: "policy-app-config",
		Manifests: []manifest{
			{Path: path.Join(tmpDir, "configmap.yaml")},
		},
	}
	p.Policies = append(p.Policies, policyConf)
	p.applyDefaults()

	err := p.createPolicy(&p.Policies[0])
	if err != nil {
		t.Fatal(err.Error())
	}

	output := p.outputBuffer.String()
	expected := `
---
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
            metadata:
                name: policy-app-config
            spec:
                object-templates:
                    - complianceType: musthave
                      objectDefinition:
                        apiVersion: v1
                        data:
                            game.properties: enemies=potato
                        kind: ConfigMap
                        metadata:
                            name: my-configmap
                remediationAction: inform
                severity: low
    remediationAction: inform
`
	expected = strings.TrimPrefix(expected, "\n")
	assertEqual(t, output, expected)
}

func TestCreatePolicyDir(t *testing.T) {
	tmpDir := t.TempDir()
	createConfigMap(t, tmpDir, "configmap.yaml")
	createConfigMap(t, tmpDir, "configmap2.yaml")
	p := Plugin{}
	p.PolicyDefaults.Namespace = "my-policies"
	policyConf := policyConfig{
		Name:      "policy-app-config",
		Manifests: []manifest{{Path: tmpDir}},
	}
	p.Policies = append(p.Policies, policyConf)
	p.applyDefaults()

	err := p.createPolicy(&p.Policies[0])
	if err != nil {
		t.Fatal(err.Error())
	}

	output := p.outputBuffer.String()
	expected := `
---
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
            metadata:
                name: policy-app-config
            spec:
                object-templates:
                    - complianceType: musthave
                      objectDefinition:
                        apiVersion: v1
                        data:
                            game.properties: enemies=potato
                        kind: ConfigMap
                        metadata:
                            name: my-configmap
                    - complianceType: musthave
                      objectDefinition:
                        apiVersion: v1
                        data:
                            game.properties: enemies=potato
                        kind: ConfigMap
                        metadata:
                            name: my-configmap
                remediationAction: inform
                severity: low
    remediationAction: inform
`
	expected = strings.TrimPrefix(expected, "\n")
	assertEqual(t, output, expected)
}

func TestCreatePolicyInvalidYAML(t *testing.T) {
	tmpDir := t.TempDir()
	manifestPath := path.Join(tmpDir, "configmap.yaml")
	err := ioutil.WriteFile(manifestPath, []byte("$ not Yaml!"), 0666)
	if err != nil {
		t.Fatalf("Failed to create %s: %v", manifestPath, err)
	}
	p := Plugin{}
	p.PolicyDefaults.Namespace = "my-policies"
	policyConf := policyConfig{
		Name:      "policy-app-config",
		Manifests: []manifest{{Path: manifestPath}},
	}
	p.Policies = append(p.Policies, policyConf)
	p.applyDefaults()

	err = p.createPolicy(&p.Policies[0])
	if err == nil {
		t.Fatal("Expected an error but did not get one")
	}

	assertEqual(t, err.Error(), "the input manifests must be in the format of YAML objects")
}

func TestCreatePlacementRuleDefault(t *testing.T) {
	p := Plugin{}
	p.PolicyDefaults.Namespace = "my-policies"
	policyConf := policyConfig{Name: "policy-app-config"}

	name, err := p.createPlacementRule(&policyConf)
	if err != nil {
		t.Fatal(err.Error())
	}

	assertEqual(t, name, "placement-policy-app-config")
	output := p.outputBuffer.String()
	expected := `
---
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
        matchExpressions: []
`
	expected = strings.TrimPrefix(expected, "\n")
	assertEqual(t, output, expected)
}

func TestCreatePlacementRuleClusterSelectors(t *testing.T) {
	p := Plugin{}
	p.PolicyDefaults.Namespace = "my-policies"
	policyConf := policyConfig{Name: "policy-app-config"}
	policyConf.Placement.ClusterSelectors = map[string]string{
		"cloud": "red hat",
		"game":  "pacman",
	}

	name, err := p.createPlacementRule(&policyConf)
	if err != nil {
		t.Fatal(err.Error())
	}

	assertEqual(t, name, "placement-policy-app-config")
	output := p.outputBuffer.String()
	expected := `
---
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
            - key: game
              operator: In
              values:
                - pacman
`
	expected = strings.TrimPrefix(expected, "\n")
	assertEqual(t, output, expected)
}

func plrPathHelper(t *testing.T, plrYAML string) (*Plugin, string) {
	tmpDir := t.TempDir()
	plrPath := path.Join(tmpDir, "plr.yaml")
	plrYAML = strings.TrimPrefix(plrYAML, "\n")
	err := ioutil.WriteFile(plrPath, []byte(plrYAML), 0666)
	if err != nil {
		t.Fatal(err.Error())
	}

	p := Plugin{}
	p.PolicyDefaults.Namespace = "my-policies"
	policyConf := policyConfig{Name: "policy-app-config"}
	policyConf.Placement.PlacementRulePath = plrPath
	p.Policies = append(p.Policies, policyConf)

	return &p, plrPath
}

func TestCreatePlacementRulePlrPath(t *testing.T) {
	plrYAML := `
---
apiVersion: apps.open-cluster-management.io/v1
kind: PlacementRule
metadata:
    name: my-plr
    namespace: my-policies
spec:
    clusterConditions:
        - status: "True"
          type: ManagedClusterConditionAvailable
    clusterSelector:
        matchExpressions:
            - key: game
              operator: In
              values:
                - pacman
`
	plrYAML = strings.TrimPrefix(plrYAML, "\n")
	p, _ := plrPathHelper(t, plrYAML)

	name, err := p.createPlacementRule(&p.Policies[0])
	if err != nil {
		t.Fatal(err.Error())
	}

	assertEqual(t, name, "my-plr")
	output := p.outputBuffer.String()
	assertEqual(t, output, plrYAML)
}

func TestCreatePlacementRulePlrPathNoName(t *testing.T) {
	plrYAML := `
---
apiVersion: apps.open-cluster-management.io/v1
kind: PlacementRule
metadata:
    namespace: my-policies
spec:
    clusterConditions:
        - status: "True"
          type: ManagedClusterConditionAvailable
    clusterSelector:
        matchExpressions: []
`
	p, plrPath := plrPathHelper(t, plrYAML)

	_, err := p.createPlacementRule(&p.Policies[0])
	if err == nil {
		t.Fatal("Expected an error but did not get one")
	}

	expected := fmt.Sprintf("the placement %s must have a name set", plrPath)
	assertEqual(t, err.Error(), expected)
}

func TestCreatePlacementRulePlrPathNoNamespace(t *testing.T) {
	plrYAML := `
---
apiVersion: apps.open-cluster-management.io/v1
kind: PlacementRule
metadata:
    name: my-plr
spec:
    clusterConditions:
        - status: "True"
          type: ManagedClusterConditionAvailable
    clusterSelector:
        matchExpressions: []
`
	p, plrPath := plrPathHelper(t, plrYAML)

	_, err := p.createPlacementRule(&p.Policies[0])
	if err == nil {
		t.Fatal("Expected an error but did not get one")
	}

	expected := fmt.Sprintf("the placement %s must have a namespace set", plrPath)
	assertEqual(t, err.Error(), expected)
}

func TestCreatePlacementRulePlrPathWrongNamespace(t *testing.T) {
	plrYAML := `
---
apiVersion: apps.open-cluster-management.io/v1
kind: PlacementRule
metadata:
    name: my-plr
    namespace: wrong-namespace
spec:
    clusterConditions:
        - status: "True"
          type: ManagedClusterConditionAvailable
    clusterSelector:
        matchExpressions: []
`
	p, plrPath := plrPathHelper(t, plrYAML)

	_, err := p.createPlacementRule(&p.Policies[0])
	if err == nil {
		t.Fatal("Expected an error but did not get one")
	}

	expected := fmt.Sprintf(
		"the placement %s must have the same namespace as the policy (%s)",
		plrPath,
		p.PolicyDefaults.Namespace,
	)
	assertEqual(t, err.Error(), expected)
}

func TestCreatePlacementRulePlrPathNoPlr(t *testing.T) {
	plrYAML := `
apiVersion: v1
kind: ConfigMap
metadata:
  name: my-configmap2
  namespace: my-policies
data:
  game.properties: |
    enemies=potato
`
	p, plrPath := plrPathHelper(t, plrYAML)

	_, err := p.createPlacementRule(&p.Policies[0])
	if err == nil {
		t.Fatal("Expected an error but did not get one")
	}

	expected := fmt.Sprintf("the placement manifest %s did not have a placement rule", plrPath)
	assertEqual(t, err.Error(), expected)
}

func TestCreatePlacementBinding(t *testing.T) {
	p := Plugin{}
	p.PolicyDefaults.Namespace = "my-policies"
	policyConf := policyConfig{Name: "policy-app-config"}
	p.Policies = append(p.Policies, policyConf)
	policyConf2 := policyConfig{Name: "policy-app-config2"}
	p.Policies = append(p.Policies, policyConf2)

	bindingName := "my-placement-binding"
	plrName := "my-placement-rule"
	policyConfs := []*policyConfig{}
	policyConfs = append(policyConfs, &p.Policies[0])
	policyConfs = append(policyConfs, &p.Policies[1])

	err := p.createPlacementBinding(bindingName, plrName, policyConfs)
	if err != nil {
		t.Fatal(err)
	}

	expected := `
---
apiVersion: policy.open-cluster-management.io/v1
kind: PlacementBinding
metadata:
    name: my-placement-binding
    namespace: my-policies
placementRef:
    apiGroup: apps.open-cluster-management.io/v1
    kind: PlacementRule
    name: my-placement-rule
subjects:
    - apiGroup: policy.open-cluster-management.io/v1
      kind: Policy
      name: policy-app-config
    - apiGroup: policy.open-cluster-management.io/v1
      kind: Policy
      name: policy-app-config2
`
	expected = strings.TrimPrefix(expected, "\n")
	assertEqual(t, p.outputBuffer.String(), expected)
}
