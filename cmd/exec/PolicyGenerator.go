package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"strings"

	"github.com/spf13/pflag"
	"gopkg.in/yaml.v3"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const policyAPIVersion = "policy.open-cluster-management.io/v1"
const policyKind = "Policy"
const configPolicyKind = "ConfigurationPolicy"
const placementRuleAPIVersion = "apps.open-cluster-management.io/v1"
const placementRuleKind = "PlacementRule"
const placementBindingAPIVersion = "policy.open-cluster-management.io/v1"
const placementBindingKind = "PlacementBinding"

var debug = false

type policyConfig struct {
	Categories []string `json:"categories,omitempty" yaml:"categories,omitempty"`
	Controls   []string `json:"controls,omitempty" yaml:"controls,omitempty"`
	Disabled   bool     `json:"disabled,omitempty" yaml:"disabled,omitempty"`
	// Make this a slice of structs in the event we want additional configuration related to
	// a manifest such as accepting patches.
	Manifests []struct {
		Path string `json:"path,omitempty" yaml:"path,omitempty"`
	} `json:"manifests,omitempty" yaml:"manifests,omitempty"`
	Name string `json:"name,omitempty" yaml:"name,omitempty"`
	// This is named Placement so that eventually PlacementRules and Placements will be supported
	Placement struct {
		ClusterSelectors  map[string]string `json:"clusterSelectors,omitempty" yaml:"clusterSelectors,omitempty"`
		PlacementRulePath string            `json:"placementRulePath,omitempty" yaml:"placementRulePath,omitempty"`
	} `json:"placement,omitempty" yaml:"placement,omitempty"`
	RemediationAction string   `json:"remediationAction,omitempty" yaml:"remediationAction,omitempty"`
	Severity          string   `json:"severity,omitempty" yaml:"severity,omitempty"`
	Standards         []string `json:"standards,omitempty" yaml:"standards,omitempty"`
}

type plugin struct {
	Metadata struct {
		Name string `json:"name,omitempty" yaml:"name,omitempty"`
	} `json:"metadata,omitempty" yaml:"metadata,omitempty"`
	PlacementBindingDefaults struct {
		Name string `json:"name,omitempty" yaml:"name,omitempty"`
	} `json:"placementBindingDefaults,omitempty" yaml:"placementBindingDefaults,omitempty"`
	PolicyDefaults struct {
		Categories []string `json:"categories,omitempty" yaml:"categories,omitempty"`
		Controls   []string `json:"controls,omitempty" yaml:"controls,omitempty"`
		Namespace  string   `json:"namespace,omitempty" yaml:"namespace,omitempty"`
		// This is named Placement so that eventually PlacementRules and Placements will be supported
		Placement struct {
			ClusterSelectors  map[string]string `json:"clusterSelectors,omitempty" yaml:"clusterSelectors,omitempty"`
			PlacementRulePath string            `json:"placementRulePath,omitempty" yaml:"placementRulePath,omitempty"`
		} `json:"placement,omitempty" yaml:"placement,omitempty"`
		RemediationAction string   `json:"remediationAction,omitempty" yaml:"remediationAction,omitempty"`
		Severity          string   `json:"severity,omitempty" yaml:"severity,omitempty"`
		Standards         []string `json:"standards,omitempty" yaml:"standards,omitempty"`
	} `json:"policyDefaults,omitempty" yaml:"policyDefaults,omitempty"`
	Policies     []policyConfig `json:"policies" yaml:"policies"`
	outputBuffer bytes.Buffer
}

func main() {
	// Parse command input
	debugFlag := pflag.Bool("debug", false, "Print the stack trace with error messages")
	standaloneFlag := pflag.Bool("standalone", false, "Run the generator binary outside of Kustomize")
	pflag.Parse()
	debug = *debugFlag
	standalone := *standaloneFlag
	argpaths := pflag.Args()

	// Handle 'kustomize build' vs running the binary 'PolicyGenerator' directly, since
	// kustomize runs the binary with the PolicyGenerator manifest as the first argument:
	// path/to/plugin/PolicyGenerator tmp/dir/cached-manifest <args>
	index := 1
	if standalone {
		index = 0
	}

	// Collect and parse PolicyGeneratorConfig file paths
	generators := argpaths[index:]
	var outputBuffer bytes.Buffer

	for _, argpath := range generators {
		parseDir(argpath, &outputBuffer)
	}

	// Output results to stdout for Kustomize to handle
	fmt.Println(outputBuffer.String())
}

func errorAndExit(msg string, formatArgs ...interface{}) {
	printArgs := make([]interface{}, len(formatArgs))
	copy(printArgs, formatArgs)
	// Show trace if the debug flag is set
	if msg == "" || debug {
		panic(fmt.Sprintf(msg, printArgs...))
	}
	fmt.Fprintf(os.Stderr, msg, printArgs...)
	fmt.Fprint(os.Stderr, "\n")
	os.Exit(1)
}

func parseDir(pathname string, outputBuffer *bytes.Buffer) {
	dir, err := os.ReadDir(pathname)
	p := plugin{}
	if err != nil {
		// Path was not a directory--return file
		outputBuffer.Write(p.ReadGeneratorConfig(pathname))
	}
	// Path is a directory--parse through its files
	for _, entry := range dir {
		filePath := path.Join(pathname, entry.Name())
		if entry.IsDir() {
			parseDir(filePath, outputBuffer)
		} else {
			outputBuffer.Write(p.ReadGeneratorConfig(filePath))
		}
	}
}

func (p *plugin) ReadGeneratorConfig(filePath string) []byte {
	fileData, err := os.ReadFile(filePath)
	if err != nil {
		errorAndExit("failed to read file '%s': %s", filePath, err)
	}

	err = p.Config(fileData)
	if err != nil {
		errorAndExit("error parsing config file '%s': %s", filePath, err)
	}

	generatedOutput, err := p.Generate()
	if err != nil {
		errorAndExit("error generating policies from config file '%s': %s", filePath, err)
	}

	return generatedOutput
}

func (p *plugin) Config(config []byte) error {
	err := yaml.Unmarshal(config, p)
	if err != nil {
		return err
	}

	p.applyDefaults()
	return p.assertValidConfig()
}

func (p *plugin) Generate() ([]byte, error) {
	for i := range p.Policies {
		err := p.createPolicy(&p.Policies[i])
		if err != nil {
			return nil, err
		}
	}

	plrNameToPolicyIdxs := map[string][]int{}
	for i := range p.Policies {
		plrName, err := p.getOrCreatePlacementRule(&p.Policies[i])
		if err != nil {
			return nil, err
		}
		plrNameToPolicyIdxs[plrName] = append(plrNameToPolicyIdxs[plrName], i)
	}

	plcBindingCount := 0
	for plrName, policyIdxs := range plrNameToPolicyIdxs {
		plcBindingCount += 1
		policyConfs := []*policyConfig{}
		for i := range policyIdxs {
			policyConfs = append(policyConfs, &p.Policies[i])
		}

		var bindingName string
		if plcBindingCount == 1 {
			bindingName = p.PlacementBindingDefaults.Name
		} else {
			bindingName = fmt.Sprintf("%s%d", p.PlacementBindingDefaults.Name, plcBindingCount)
		}

		p.createPlacementBinding(bindingName, plrName, policyConfs)
	}

	return p.outputBuffer.Bytes(), nil
}

func (p *plugin) applyDefaults() {
	if len(p.Policies) == 0 {
		return
	}

	if p.PlacementBindingDefaults.Name == "" && len(p.Policies) == 1 {
		p.PlacementBindingDefaults.Name = "binding-" + p.Policies[0].Name
	}

	// Set defaults to the defaults that aren't overridden
	if p.PolicyDefaults.Categories == nil {
		p.PolicyDefaults.Categories = []string{"CM Configuration Management"}
	}

	if p.PolicyDefaults.Controls == nil {
		p.PolicyDefaults.Controls = []string{"CM-2 Baseline Configuration"}
	}

	if p.PolicyDefaults.RemediationAction == "" {
		p.PolicyDefaults.RemediationAction = "inform"
	}

	if p.PolicyDefaults.Severity == "" {
		p.PolicyDefaults.Severity = "low"
	}

	if p.PolicyDefaults.Standards == nil {
		p.PolicyDefaults.Standards = []string{"NIST SP 800-53"}
	}

	for i := range p.Policies {
		if p.Policies[i].Categories == nil {
			p.Policies[i].Categories = p.PolicyDefaults.Categories
		}

		if p.Policies[i].Controls == nil {
			p.Policies[i].Controls = p.PolicyDefaults.Controls
		}

		if p.Policies[i].Placement.ClusterSelectors == nil {
			p.Policies[i].Placement.ClusterSelectors = p.PolicyDefaults.Placement.ClusterSelectors
		}

		if p.Policies[i].Placement.PlacementRulePath == "" {
			p.Policies[i].Placement.PlacementRulePath = p.PolicyDefaults.Placement.PlacementRulePath
		}

		if p.Policies[i].RemediationAction == "" {
			p.Policies[i].RemediationAction = p.PolicyDefaults.RemediationAction
		}

		if p.Policies[i].Severity == "" {
			p.Policies[i].Severity = p.PolicyDefaults.Severity
		}

		if p.Policies[i].Standards == nil {
			p.Policies[i].Standards = p.PolicyDefaults.Standards
		}
	}

}

// assertValidConfig verifies that the user provided configuration has all the
// required fields. Note that this should be run only after applyDefaults is run.
func (p *plugin) assertValidConfig() error {
	if p.PlacementBindingDefaults.Name == "" {
		return errors.New(
			"placementBindingDefaults.name must be set when there are mutiple policies",
		)
	}

	if p.PolicyDefaults.Namespace == "" {
		return errors.New("policyDefaults.namespace is empty but it must be set")
	}

	if len(p.Policies) == 0 {
		return errors.New("policies is empty but it must be set")
	}

	for i := range p.Policies {
		if len(p.Policies[i].Manifests) == 0 {
			return errors.New("each policy must have at least one manifest")
		}

		for _, manifest := range p.Policies[i].Manifests {
			if manifest.Path == "" {
				return errors.New("each policy manifest entry must have path set")
			}

			_, err := os.Stat(manifest.Path)
			if err != nil {
				return fmt.Errorf("could not read the manifest path %s", manifest.Path)
			}
		}

		if p.Policies[i].Name == "" {
			return errors.New("each policy must have a name set")
		}
	}

	return nil
}

func (p *plugin) createPolicy(policyConf *policyConfig) error {

	policyTemplate, err := getPolicyTemplate(policyConf)
	if err != nil {
		return err
	}

	policy := map[string]interface{}{
		"apiVersion": policyAPIVersion,
		"kind":       policyKind,
		"metadata": map[string]interface{}{
			"annotations": map[string]string{
				"policy.open-cluster-management.io/categories": strings.Join(policyConf.Categories, ","),
				"policy.open-cluster-management.io/controls":   strings.Join(policyConf.Controls, ","),
				"policy.open-cluster-management.io/standards":  strings.Join(policyConf.Standards, ","),
			},
			"name":      policyConf.Name,
			"namespace": p.PolicyDefaults.Namespace,
		},
		"spec": map[string]interface{}{
			"disabled":          policyConf.Disabled,
			"policy-templates":  []map[string]map[string]interface{}{*policyTemplate},
			"remediationAction": policyConf.RemediationAction,
		},
	}

	policyYAML, err := yaml.Marshal(policy)
	if err != nil {
		return fmt.Errorf(
			"an unexpected error occurred when converting the policy to YAML: %w", err,
		)
	}

	p.outputBuffer.Write([]byte("---\n"))
	p.outputBuffer.Write(policyYAML)
	return nil
}

func getPolicyTemplate(policyConf *policyConfig) (
	*map[string]map[string]interface{}, error,
) {
	manifests := []interface{}{}
	for _, manifest := range policyConf.Manifests {
		manifestPaths := []string{}
		readErr := fmt.Errorf("failed to read the manifest directory %s", manifest.Path)
		manifestPathInfo, err := os.Stat(manifest.Path)
		if err != nil {
			return nil, readErr
		}

		if manifestPathInfo.IsDir() {
			files, err := ioutil.ReadDir(manifest.Path)
			if err != nil {
				return nil, readErr
			}

			for _, f := range files {
				if f.IsDir() {
					continue
				}

				ext := path.Ext(f.Name())
				if ext != ".yaml" && ext != ".yml" {
					continue
				}

				yamlPath := path.Join(manifest.Path, f.Name())
				manifestPaths = append(manifestPaths, yamlPath)
			}
		} else {
			manifestPaths = append(manifestPaths, manifest.Path)
		}

		for _, manifestPath := range manifestPaths {
			manifestFile, err := unmarshalManifestFile(manifestPath)
			if err != nil {
				return nil, err
			}

			if len(*manifestFile) == 0 {
				continue
			}

			manifests = append(manifests, *manifestFile...)
		}
	}

	if len(manifests) == 0 {
		return nil, fmt.Errorf(
			"the policy %s must specify at least one non-empty manifest file", policyConf.Name,
		)
	}

	policyTemplate := map[string]map[string]interface{}{
		"objectDefinition": {
			"apiVersion": policyAPIVersion,
			"kind":       configPolicyKind,
			"name":       policyConf.Name,
			"spec": map[string]interface{}{
				"remediationAction": policyConf.RemediationAction,
				"severity":          policyConf.Severity,
				"object-templates":  manifests,
			},
		},
	}

	return &policyTemplate, nil
}

func (p *plugin) getOrCreatePlacementRule(policyConf *policyConfig) (name string, err error) {
	plrPath := policyConf.Placement.PlacementRulePath
	if plrPath != "" {
		var manifests *[]interface{}
		manifests, err = unmarshalManifestFile(plrPath)
		if err != nil {
			return
		}

		for _, manifest := range *manifests {
			var object = manifest.(map[string]interface{})
			if kind, _, _ := unstructured.NestedString(object, "kind"); kind != placementRuleKind {
				continue
			}

			var found bool
			name, found, err = unstructured.NestedString(object, "metadata", "name")
			if !found || err != nil {
				err = fmt.Errorf("the placement %s must have a name set", plrPath)
				return
			}

			var namespace string
			namespace, found, err = unstructured.NestedString(object, "metadata", "namespace")
			if !found || err != nil {
				err = fmt.Errorf("the placement %s must have a namespace set", plrPath)
				return
			}

			if namespace != p.PolicyDefaults.Namespace {
				err = fmt.Errorf(
					"the placement %s must have the same namespace as the policy (%s)",
					plrPath,
					p.PolicyDefaults.Namespace,
				)
				return
			}

			break
		}

		if name == "" {
			err = fmt.Errorf(
				"the placement manifest %s did not have a placement rule", plrPath,
			)

			return
		}

		return
	}

	matchExpressions := []map[string]interface{}{}
	for label, value := range policyConf.Placement.ClusterSelectors {
		matchExpression := map[string]interface{}{
			"key":      label,
			"operator": "In",
			"values":   []string{value},
		}
		matchExpressions = append(matchExpressions, matchExpression)
	}

	name = "placement-" + policyConf.Name
	rule := map[string]interface{}{
		"apiVersion": placementRuleAPIVersion,
		"kind":       placementRuleKind,
		"metadata": map[string]interface{}{
			"name":      name,
			"namespace": p.PolicyDefaults.Namespace,
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

	var ruleYAML []byte
	ruleYAML, err = yaml.Marshal(rule)
	if err != nil {
		err = fmt.Errorf(
			"an unexpected error occurred when converting the placement rule to YAML: %w", err,
		)

		return
	}

	p.outputBuffer.Write([]byte("---\n"))
	p.outputBuffer.Write(ruleYAML)

	return
}

func (p *plugin) createPlacementBinding(
	bindingName, plrName string, policyConfs []*policyConfig,
) error {
	subjects := make([]map[string]string, 0, len(policyConfs))
	for _, policyConf := range policyConfs {
		subject := map[string]string{
			"apiGroup": policyAPIVersion,
			"kind":     policyKind,
			"name":     policyConf.Name,
		}
		subjects = append(subjects, subject)
	}

	binding := map[string]interface{}{
		"apiVersion": placementBindingAPIVersion,
		"kind":       placementBindingKind,
		"metadata": map[string]interface{}{
			"name":      bindingName,
			"namespace": p.PolicyDefaults.Namespace,
		},
		"placementRef": map[string]string{
			"apiGroup": placementRuleAPIVersion,
			"name":     plrName,
			"kind":     placementRuleKind,
		},
		"subjects": subjects,
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
