package main

import (
	"bytes"
	"fmt"
	"os"
	"path"

	"github.com/mprahl/policygenerator/internal"
	"github.com/spf13/pflag"
)

var debug = false

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
	if err != nil {
		// Path was not a directory--return file
		outputBuffer.Write(ReadGeneratorConfig(pathname))
	}
	// Path is a directory--parse through its files
	for _, entry := range dir {
		filePath := path.Join(pathname, entry.Name())
		if entry.IsDir() {
			parseDir(filePath, outputBuffer)
		} else {
			outputBuffer.Write(ReadGeneratorConfig(filePath))
		}
	}
}

func ReadGeneratorConfig(filePath string) []byte {
	p := internal.Plugin{}
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
