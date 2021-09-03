package main

import (
	"bytes"
	"fmt"
	"os"
	"path"

	"github.com/mprahl/policygenerator/internal"
)

func main() {
	// When executing with `kustomize build`
	index := 2
	// When executing with `./PolicyGenerator` directly
	if os.Args[0] == "./PolicyGenerator" {
		index = 1
	}
	argpaths := os.Args[index:]
	var outputBuffer bytes.Buffer

	for _, argpath := range argpaths {
		dir, err := os.ReadDir(argpath)
		if err != nil {
			p := internal.Plugin{}
			file, err := os.ReadFile(argpath)
			if err != nil {
				panic(err)
			}
			err = p.Config(file)
			if err != nil {
				panic(err)
			}
			output, err := p.Generate()
			if err != nil {
				panic(err)
			}
			outputBuffer.Write(output)
		} else {
			for _, entry := range dir {
				if entry.IsDir() {
					continue
				}
				file, err := os.ReadFile(path.Join(argpath, entry.Name()))
				if err != nil {
					panic(err)
				}
				p := internal.Plugin{}
				err = p.Config(file)
				if err != nil {
					panic(err)
				}
				output, err := p.Generate()
				if err != nil {
					panic(err)
				}
				outputBuffer.Write(output)
			}
		}
	}
	fmt.Println(outputBuffer.String())
}
