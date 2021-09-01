package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"regexp"

	"gopkg.in/yaml.v3"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"sigs.k8s.io/kustomize/api/resmap"
)

const kustomizeDir = "kustomization"
const policyAPIVersion = "policy.open-cluster-management.io/v1"
const policyKind = "Policy"
const configPolicyKind = "ConfigurationPolicy"
const placementRuleAPIVersion = "apps.open-cluster-management.io/v1"
const placementRuleKind = "PlacementRule"
const placementBindingAPIVersion = "policy.open-cluster-management.io/v1"
const placementBindingKind = "PlacementBinding"
const basePatchFilename = "base-patch.yaml"

var clusterSelectorRegex = regexp.MustCompile(`^(.+)=(.+)$`)

// TODO: Should all the flags be accepted here?
type plugin struct {
	rf               *resmap.Factory
	ClusterSelectors map[string]string `json:"clusterSelectors,omitempty" yaml:"clusterSelectors,omitempty"`
	Manifests        []string          `json:"manifests,omitempty" yaml:"manifests,omitempty"`
	// Define the struct rather than embed ObjectMeta in order to just accept name and namespace
	Metadata struct {
		Name      string `json:"name,omitempty" yaml:"name,omitempty"`
		Namespace string `json:"namespace,omitempty" yaml:"namespace,omitempty"`
	} `json:"metadata,omitempty" yaml:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`
	// This is named Placement so that eventually PlacementRules and Placements will be supported
	Placement    string `json:"placement,omitempty" yaml:"placement,omitempty"`
	outputBuffer bytes.Buffer
}

//nolint: golint
//noinspection GoUnusedGlobalVariable
var KustomizePlugin plugin

func (p *plugin) Config(h *resmap.PluginHelpers, config []byte) error {
	p.rf = h.ResmapFactory()
	err := yaml.Unmarshal(config, p)
	if err != nil {
		return err
	}

	return p.assertValidFlags()
}

func (p *plugin) Generate() (resmap.ResMap, error) {
	err := p.createPolicy()
	if err != nil {
		return nil, err
	}

	placementRuleName, err := p.createPlacementRule()
	if err != nil {
		return nil, err
	}

	err = p.createPlacementBinding(placementRuleName)
	if err != nil {
		return nil, err
	}

	return p.rf.NewResMapFromBytes(p.outputBuffer.Bytes())
}

func (p *plugin) assertValidFlags() error {
	if p.Metadata.Name == "" {
		return errors.New("metadata.name must be set")
	}

	if p.Metadata.Namespace == "" {
		return errors.New("metadata.namespace must be set")
	}

	if p.Placement != "" {
		if _, err := os.Stat(p.Placement); err != nil {
			return fmt.Errorf("the placement at %s could not be read", p.Placement)
		}
	}

	for _, m := range p.Manifests {
		if _, err := os.Stat(m); err != nil {
			return fmt.Errorf("the object manifest at %s could not be read", m)
		}
	}

	return nil
}

func (p *plugin) createPolicy() error {

	policyTemplate, err := getPolicyTemplate(p.Manifests)
	if err != nil {
		return err
	}

	policy := map[string]interface{}{
		"apiVersion": policyAPIVersion,
		"kind":       policyKind,
		"metadata": map[string]interface{}{
			"annotations": map[string]string{
				"policy.open-cluster-management.io/categories": "CM Configuration Management",
				"policy.open-cluster-management.io/controls":   "CM-2 Baseline Configuration",
				"policy.open-cluster-management.io/standards":  "NIST SP 800-53",
			},
			"name":      p.Metadata.Name,
			"namespace": p.Metadata.Namespace,
		},
		"spec": map[string]interface{}{
			"disabled":          true,
			"policy-templates":  []map[string]map[string]interface{}{*policyTemplate},
			"remediationAction": "inform",
		},
	}

	policyYAML, err := yaml.Marshal(policy)
	if err != nil {
		return fmt.Errorf(
			"an unexpected error occurred when converting the policy to YAML: %w", err,
		)
	}

	p.outputBuffer.Write(policyYAML)
	return nil
}

func getPolicyTemplate(manifestPaths []string) (*map[string]map[string]interface{}, error) {
	manifests := []interface{}{}
	for _, manifestPath := range manifestPaths {
		manifestFile, err := unmarshalManifestFile(manifestPath)
		if err != nil {
			return nil, err
		}

		if len(*manifestFile) == 0 {
			return nil, errors.New("manifest files cannot be empty")
		}

		manifests = append(manifests, *manifestFile...)
	}

	policyTemplate := map[string]map[string]interface{}{
		"objectDefinition": {
			"apiVersion": policyAPIVersion,
			"kind":       configPolicyKind,
			"spec": map[string]interface{}{
				"remediationAction": "inform",
				"severity":          "low",
				"object-templates":  manifests,
			},
		},
	}

	return &policyTemplate, nil
}

func (p *plugin) createPlacementRule() (string, error) {
	if p.Placement != "" {
		manifests, err := unmarshalManifestFile(p.Placement)
		if err != nil {
			return "", err
		}

		var placementRuleName string
		for _, manifest := range *manifests {
			var object = manifest.(map[string]interface{})
			if kind, _, _ := unstructured.NestedString(object, "kind"); kind != placementRuleKind {
				continue
			}

			var found bool
			placementRuleName, found, err = unstructured.NestedString(object, "metadata", "name")
			if !found || err != nil {
				return "", fmt.Errorf("the placement %s must have a name set", p.Placement)
			}

			break
		}

		if placementRuleName == "" {
			return "", fmt.Errorf(
				"the placement manifest %s did not have a placement rule", p.Placement,
			)
		}

		return placementRuleName, nil
	}

	matchExpressions := []map[string]interface{}{}
	for label, value := range p.ClusterSelectors {
		matchExpression := map[string]interface{}{
			"key":      label,
			"operator": "In",
			"values":   []string{value},
		}
		matchExpressions = append(matchExpressions, matchExpression)
	}

	placementRuleName := "placement-" + p.Metadata.Name
	rule := map[string]interface{}{
		"apiVersion": placementRuleAPIVersion,
		"kind":       placementRuleKind,
		"metadata": map[string]interface{}{
			"name":      placementRuleName,
			"namespace": p.Metadata.Namespace,
		},
		"spec": map[string]interface{}{
			"clusterConditions": []map[string]string{
				{"status": "True", "type": "ManagedClusterConditionAvailable"},
			},
			"clusterSelector": map[string]interface{}{
				"matchExpressions": matchExpressions,
			},
		},
	}

	ruleYAML, err := yaml.Marshal(rule)
	if err != nil {
		return "", fmt.Errorf(
			"an unexpected error occurred when converting the placement rule to YAML: %w", err,
		)
	}

	p.outputBuffer.Write([]byte("---\n"))
	p.outputBuffer.Write(ruleYAML)

	return placementRuleName, nil
}

func (p *plugin) createPlacementBinding(placementRuleName string) error {
	binding := map[string]interface{}{
		"apiVersion": placementBindingAPIVersion,
		"kind":       placementBindingKind,
		"metadata": map[string]interface{}{
			"name":      "binding-" + p.Metadata.Name,
			"namespace": p.Metadata.Namespace,
		},
		"placementRef": map[string]string{
			"name":     placementRuleName,
			"kind":     placementRuleKind,
			"apiGroup": placementRuleAPIVersion,
		},
		"subjects": []map[string]string{
			{
				"name":     p.Metadata.Name,
				"kind":     policyKind,
				"apiGroup": policyAPIVersion,
			},
		},
	}

	bindingYAML, err := yaml.Marshal(binding)
	if err != nil {
		return fmt.Errorf(
			"an unexpected error occurred when converting the placement binding to YAML: %w", err,
		)
	}

	p.outputBuffer.Write([]byte("---\n"))
	p.outputBuffer.Write(bindingYAML)

	return nil
}

// unmarshalManifestFile unmarshals the input object manifest/definition file into
// a slice in order to account for multiple YAML documents in the same file.
// If the file cannot be decoded or each document is not a map, an error will
// be returned.
func unmarshalManifestFile(manifestPath string) (*[]interface{}, error) {
	manifestBytes, err := ioutil.ReadFile(manifestPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read the manifest file %s", manifestPath)
	}

	yamlDocs := []interface{}{}
	d := yaml.NewDecoder(bytes.NewReader(manifestBytes))
	for {
		var obj interface{}
		err := d.Decode(&obj)
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}

			return nil, err
		}

		if _, ok := obj.(map[string]interface{}); !ok {
			err := errors.New("the input manifests must be in the format of YAML objects")
			return nil, err
		}

		yamlDocs = append(yamlDocs, obj)
	}

	return &yamlDocs, nil
}
