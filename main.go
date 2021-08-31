// TODO: Add placement rule and placement binding support
package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/kustomize/api/krusty"
	"sigs.k8s.io/kustomize/kyaml/filesys"
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

// Create a new type for a list of Strings
type stringList []string

// Implement the flag.Value interface
func (s *stringList) String() string {
	return fmt.Sprintf("%v", *s)
}

func (s *stringList) Set(value string) error {
	*s = strings.Split(value, ",")
	return nil
}

func getPolicyConfigBase(name, namespace string) map[string]interface{} {
	return map[string]interface{}{
		"apiVersion": "policy.open-cluster-management.io/v1",
		"kind":       "Policy",
		"metadata": map[string]interface{}{
			"name":      name,
			"namespace": namespace,
		},
	}
}

// unmarshalObjDefFile unmarshals the input object manifest/definition file into
// a slice in order to account for multiple YAML documents in the same file.
// If the file cannot be decoded or each document is not a map, an error will
// be returned.
func unmarshalObjDefFile(objDefFile []byte) (*[]interface{}, error) {
	yamlDocs := []interface{}{}
	d := yaml.NewDecoder(bytes.NewReader(objDefFile))
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
			err := errors.New("the input object manifests must be in the format of YAML objects")
			return nil, err
		}

		yamlDocs = append(yamlDocs, obj)
	}

	return &yamlDocs, nil
}

func createPatchFromK8sObjects(
	name,
	namespace,
	remAction,
	severity string,
	annotations *map[string]string,
	disabled bool,
	objDefFiles *[][]byte,
) ([]byte, error) {
	objDefYamls := []interface{}{}
	for _, objDefFile := range *objDefFiles {
		objDefs, err := unmarshalObjDefFile(objDefFile)
		if err != nil {
			return nil, err
		}

		if len(*objDefs) == 0 {
			return nil, errors.New("object manifest files cannot be empty")
		}

		objDefYamls = append(objDefYamls, *objDefs...)
	}

	policyTemplate := map[string]map[string]interface{}{
		"objectDefinition": {
			"apiVersion": policyAPIVersion,
			"kind":       configPolicyKind,
			"spec": map[string]interface{}{
				"remediationAction": remAction,
				"severity":          severity,
				"object-templates":  objDefYamls,
			},
		},
	}

	// Create a map directly instead of using the config-policy-controller Go
	// module to avoid default values being set in the patch.
	patch := map[string]interface{}{
		"apiVersion": "policy.open-cluster-management.io/v1",
		"kind":       "Policy",
		"metadata": map[string]interface{}{
			"name":        name,
			"namespace":   namespace,
			"annotations": *annotations,
		},
		"spec": map[string]interface{}{
			"remediationAction": remAction,
			"disabled":          disabled,
			"policy-templates":  []map[string]map[string]interface{}{policyTemplate},
		},
	}

	return yaml.Marshal(patch)
}

func errorAndExit(msg string, formatArgs ...interface{}) {
	printArgs := make([]interface{}, len(formatArgs))
	copy(printArgs, formatArgs)
	fmt.Fprintf(os.Stderr, msg, printArgs...)
	fmt.Fprint(os.Stderr, "\n")
	os.Exit(1)
}

func assertValidFlags(
	policyNamespace,
	policyName,
	placementPath string,
	clusterSelectors stringList,
	patches stringList,
	objDefs []string,
) {
	if policyName == "" {
		errorAndExit("The -name flag must be set")
	}

	if policyNamespace == "" {
		errorAndExit("The -namespace flag must be set")
	}

	if placementPath != "" {
		if _, err := os.Stat(placementPath); err != nil {
			errorAndExit("The placement %s could not be read", placementPath)
		}
	}

	for _, clusterSelector := range clusterSelectors {
		if matched := clusterSelectorRegex.MatchString(clusterSelector); !matched {
			errorAndExit(
				`The clusterSelector "%s" must be in the format of "label=value"`,
				clusterSelector,
			)
		}
	}

	for _, patchPath := range patches {
		if _, err := os.Stat(patchPath); err != nil {
			errorAndExit("The patch %s could not be read", patchPath)
		}
	}

	for _, objDefPath := range objDefs {
		if _, err := os.Stat(objDefPath); err != nil {
			errorAndExit("The object manifest %s could not be read", objDefPath)
		}
	}

}

func prepareKustomizationEnv(
	fSys filesys.FileSystem, patches []string, policyNamespace, policyName string,
) error {
	kustomizationYamlFile := map[string][]interface{}{
		"resources": {"configurationpolicy.yaml"},
		// Seed the dynamically created patch
		"patchesStrategicMerge": {basePatchFilename},
	}

	// Get all the patches
	for i, patchPath := range patches {
		fileBytes, err := ioutil.ReadFile(patchPath)
		if err != nil {
			return fmt.Errorf("failed to read the patch %s", patchPath)
		}

		var patchYaml map[string]interface{}
		err = yaml.Unmarshal(fileBytes, &patchYaml)
		if err != nil {
			return fmt.Errorf("the patch %s is in an invalid format", patchPath)
		}

		// Populate some required fields that were provided and are required by Kustomize
		unstructured.SetNestedField(patchYaml, policyName, "metadata", "name")
		unstructured.SetNestedField(patchYaml, policyNamespace, "metadata", "namespace")
		// Populate some fields that are required by Kustomize and always the same
		apiVersion, found, err := unstructured.NestedString(patchYaml, "apiVersion")
		if err != nil || (found && apiVersion != policyAPIVersion) {
			return fmt.Errorf(
				"patches must have apiVersion not be set or set to %s", policyAPIVersion,
			)
		} else if !found {
			unstructured.SetNestedField(patchYaml, policyAPIVersion, "apiVersion")
		}

		kind, found, err := unstructured.NestedString(patchYaml, "kind")
		if err != nil || (found && kind != policyKind) {
			return fmt.Errorf("patches must have kind not be set or set to %s", policyKind)
		} else if !found {
			unstructured.SetNestedField(patchYaml, policyKind, "kind")
		}

		fileBytes, err = yaml.Marshal(patchYaml)
		if err != nil {
			return fmt.Errorf("the modifications on patch %s failed", patchPath)
		}

		fsPath := fmt.Sprintf("userpatch%d.yaml", i+1)
		err = fSys.WriteFile(path.Join(kustomizeDir, fsPath), fileBytes)
		if err != nil {
			return fmt.Errorf("failed to load the patch %s in memory", patchPath)
		}

		kustomizationYamlFile["patchesStrategicMerge"] = append(
			kustomizationYamlFile["patchesStrategicMerge"],
			fsPath,
		)
	}

	kustomizationBytes, err := yaml.Marshal(kustomizationYamlFile)
	if err != nil {
		return err
	}

	err = fSys.WriteFile(path.Join(kustomizeDir, "kustomization.yaml"), kustomizationBytes)
	if err != nil {
		panic(err)
	}

	return nil
}

func addCommentHeader(policyYAML *[]byte) *[]byte {
	args := []string{path.Base(os.Args[0])}
	args = append(args, os.Args[1:]...)
	outputYAML := []byte(
		fmt.Sprintf(`#
# This file is autogenerated by %s
# To update, run:
#
#    %s
#
---
`,
			args[0],
			strings.Join(args, " "),
		),
	)

	outputYAML = append(outputYAML, *policyYAML...)
	return &outputYAML
}

func addPlacementObjects(
	policyYaml *[]byte,
	policyNamespace,
	policyName,
	placementPath string,
	clusterSelectors stringList,
) (*[]byte, error) {

	matchExpressions := []map[string]interface{}{}
	for _, clusterSelector := range clusterSelectors {
		// This was validated already in the assertValidFlags function
		matches := clusterSelectorRegex.FindStringSubmatch(clusterSelector)
		label := matches[1]
		value := matches[2]
		matchExpression := map[string]interface{}{
			"key":      label,
			"operator": "In",
			"values":   []string{value},
		}
		matchExpressions = append(matchExpressions, matchExpression)
	}

	combinedYAML := *policyYaml
	var placementRuleName string
	if placementPath != "" {
		placementBytes, err := ioutil.ReadFile(placementPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read %s", placementPath)
		}

		objects, err := unmarshalObjDefFile(placementBytes)
		if err != nil {
			return nil, fmt.Errorf("the placement path %s is invalid YAML: %v", placementPath, err)
		}

		for _, object := range *objects {
			var object = object.(map[string]interface{})
			if kind, _, _ := unstructured.NestedString(object, "kind"); kind != placementRuleKind {
				continue
			}

			var found bool
			placementRuleName, found, err = unstructured.NestedString(object, "metadata", "name")
			if !found || err != nil {
				return nil, fmt.Errorf("the placement path %s must have a name set", placementPath)
			}

			break
		}

		if placementRuleName == "" {
			return nil, fmt.Errorf(
				"the placement path %s did not have a placement rule", placementPath,
			)
		}
	} else {
		placementRuleName = "placement-" + policyName
		rule := map[string]interface{}{
			"apiVersion": placementRuleAPIVersion,
			"kind":       placementRuleKind,
			"metadata": map[string]interface{}{
				"name":      placementRuleName,
				"namespace": policyNamespace,
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
		// An error shouldn't be possible so panic if it is encountered
		if err != nil {
			panic(err)
		}
		ruleYAML = append([]byte("---\n"), ruleYAML...)
		combinedYAML = append(combinedYAML, ruleYAML...)
	}

	binding := map[string]interface{}{
		"apiVersion": placementBindingAPIVersion,
		"kind":       placementBindingKind,
		"metadata": map[string]interface{}{
			"name":      "binding-" + policyName,
			"namespace": policyNamespace,
		},
		"placementRef": map[string]string{
			"name":     placementRuleName,
			"kind":     placementRuleKind,
			"apiGroup": placementRuleAPIVersion,
		},
		"subjects": []map[string]string{
			{
				"name":     policyName,
				"kind":     policyKind,
				"apiGroup": policyAPIVersion,
			},
		},
	}

	bindingYAML, err := yaml.Marshal(binding)
	// An error shouldn't be possible so panic if it is encountered
	if err != nil {
		panic(err)
	}
	bindingYAML = append([]byte("---\n"), bindingYAML...)
	combinedYAML = append(combinedYAML, bindingYAML...)

	return &combinedYAML, nil
}

func main() {
	nsFlag := flag.String("namespace", "replace-me", "the namespace for the policy")
	nameFlag := flag.String("name", "replace-me", "the name for the policy")
	var clusterSelectors stringList
	flag.Var(
		&clusterSelectors,
		"cluster-selectors",
		"a comma-separated list of placement rule cluster selectors; if not provided, the "+
			"placement rule will be for all clusters; does not take effect if -placement is set",
	)
	outputFlag := flag.String("o", "", "the path to write the policy to; defaults to stdout")
	placementFlag := flag.String(
		"placement",
		"",
		"the path to the placement rule to use; takes precedence over -cluster-selectors",
	)
	var patches stringList
	flag.Var(&patches, "patches", "a comma-separated list of Kustomize-like patches")
	var categories stringList
	flag.Var(
		&categories,
		"categories",
		"a comma-separated list of the policy's categories",
	)
	var controls stringList
	flag.Var(
		&controls,
		"controls",
		"a comma-separated list of the policy's controls",
	)
	var standards stringList
	flag.Var(&standards, "standards", "a comma-separated list of the policy's standards")
	disabledFlag := flag.Bool("disabled", true, "determines if the policy is disabled")
	remediationActionFlag := flag.String(
		"remediationAction", "inform", "the policy's remediation action (inform or enforce)",
	)
	severityFlag := flag.String("severity", "low", "the policy's severity (high, medium, or low)")
	flag.Parse()

	assertValidFlags(*nsFlag, *nameFlag, *placementFlag, clusterSelectors, patches, flag.Args())

	policyNamespace := *nsFlag
	policyName := *nameFlag
	outputPath := *outputFlag
	policyDisabled := *disabledFlag
	policyRemAction := *remediationActionFlag
	policySeverity := *severityFlag
	placementPath := *placementFlag
	objDefPaths := flag.Args()

	if len(categories) == 0 {
		categories = stringList{"CM Configuration Management"}
	}

	if len(controls) == 0 {
		controls = stringList{"CM-2 Baseline Configuration"}
	}

	if len(standards) == 0 {
		standards = stringList{"NIST SP 800-53"}
	}

	policyAnnotations := map[string]string{
		"policy.open-cluster-management.io/categories": strings.Join(categories, ","),
		"policy.open-cluster-management.io/controls":   strings.Join(controls, ","),
		"policy.open-cluster-management.io/standards":  strings.Join(standards, ","),
	}

	k := krusty.MakeKustomizer(krusty.MakeDefaultOptions())
	// Create the file system in memory with the Kustomize YAML files
	fSys := filesys.MakeFsInMemory()
	fSys.Mkdir(kustomizeDir)

	configPolicyBase := getPolicyConfigBase(policyName, policyNamespace)
	configPolicyBaseBytes, err := yaml.Marshal(configPolicyBase)
	if err != nil {
		errorAndExit("Failed to convert the configuration policy to YAML")
	}

	err = fSys.WriteFile(path.Join(kustomizeDir, "configurationpolicy.yaml"), configPolicyBaseBytes)
	if err != nil {
		errorAndExit("Failed to load the create configuration policy YAML file in memory: %v", err)
	}

	objDefsBytes := [][]byte{}
	for _, objDefPath := range objDefPaths {
		objDefBytes, err := ioutil.ReadFile(objDefPath)
		if err != nil {
			errorAndExit("Failed to read %s", objDefPath)
		}

		objDefsBytes = append(objDefsBytes, objDefBytes)
	}

	patch, err := createPatchFromK8sObjects(
		policyName,
		policyNamespace,
		policyRemAction,
		policySeverity,
		&policyAnnotations,
		policyDisabled,
		&objDefsBytes,
	)
	if err != nil {
		errorAndExit("Failed to create a policy: %v", err)
	}

	err = fSys.WriteFile(path.Join(kustomizeDir, basePatchFilename), patch)
	if err != nil {
		errorAndExit("Failed to load %s in memory: %v", basePatchFilename, err)
	}

	err = prepareKustomizationEnv(fSys, patches, policyNamespace, policyName)
	if err != nil {
		// Indexing is safe here since the error message is always ASCII
		errMsg := strings.ToUpper(string(err.Error()[0])) + string(err.Error()[1:])
		errorAndExit(errMsg)
	}

	m, err := k.Run(fSys, kustomizeDir)
	if err != nil {
		errorAndExit("Executing kustomize failed: %v", err)
	}

	policyYAML, err := m.AsYaml()
	if err != nil {
		errorAndExit("Could not convert the configuration policy to YAML: %v", err)
	}

	allYAML := addCommentHeader(&policyYAML)
	allYAML, err = addPlacementObjects(
		allYAML, policyNamespace, policyName, placementPath, clusterSelectors,
	)

	if err != nil {
		errorAndExit("Failed to generate the placement binding/rule: %v", err)
	}

	if outputPath != "" {
		os.WriteFile(outputPath, *allYAML, 0444)
	} else {
		fmt.Println(string(*allYAML))
	}
}
