// TODO: Add placement rule and placement binding support
package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"path"
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

func createPatchFromK8sObjects(
	name,
	namespace,
	remAction,
	severity string,
	annotations map[string]string,
	disabled bool,
	objDefs [][]byte,
) ([]byte, error) {
	objDefYamls := []interface{}{}
	for _, objDef := range objDefs {
		var obj interface{}
		err := yaml.Unmarshal(objDef, &obj)
		if err != nil {
			return nil, err
		}

		objDefYamls = append(objDefYamls, obj)
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
			"annotations": annotations,
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
	policyName string,
	patches stringList,
	objDefs []string,
) {
	if policyName == "" {
		errorAndExit("The -name flag must be set")
	}

	if policyNamespace == "" {
		errorAndExit("The -namespace flag must be set")
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
		"patchesStrategicMerge": {"patch.yaml"},
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
		unstructured.SetNestedField(patchYaml, policyAPIVersion, "apiVersion")
		unstructured.SetNestedField(patchYaml, policyKind, "kind")

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

func main() {
	nsFlag := flag.String("namespace", "replace-me", "the namespace for the policy")
	nameFlag := flag.String("name", "replace-me", "the name for the policy")
	outputFlag := flag.String("o", "", "the path to write the policy to; defaults to stdout")
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

	assertValidFlags(*nsFlag, *nameFlag, patches, flag.Args())

	policyNamespace := *nsFlag
	policyName := *nameFlag
	outputPath := *outputFlag
	policyDisabled := *disabledFlag
	policyRemAction := *remediationActionFlag
	policySeverity := *severityFlag
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
		policyAnnotations,
		policyDisabled,
		objDefsBytes,
	)
	if err != nil {
		errorAndExit("Failed to create a policy: %v", err)
	}

	err = fSys.WriteFile(path.Join(kustomizeDir, "patch.yaml"), patch)
	if err != nil {
		errorAndExit("Failed to load patch.yaml in memory: %v", err)
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

	yaml, err := m.AsYaml()
	if err != nil {
		errorAndExit("Could not convert the configuration policy to YAML", err)
	}

	if outputPath != "" {
		os.WriteFile(outputPath, yaml, 0444)
	} else {
		fmt.Println(string(yaml))
	}
}
