package main

import (
	"github.com/mprahl/policygenerator/internal"
	"sigs.k8s.io/kustomize/api/resmap"
)

//nolint: golint
//noinspection GoUnusedGlobalVariable
var KustomizePlugin kustomizePlugin

type kustomizePlugin struct {
	rf *resmap.Factory
	internal.Plugin
}

func (p *kustomizePlugin) Config(h *resmap.PluginHelpers, config []byte) error {
	p.rf = h.ResmapFactory()
	return p.Plugin.Config(config)
}

func (p *kustomizePlugin) Generate() (resmap.ResMap, error) {
	output, err := p.Plugin.Generate()
	if err != nil {
		return nil, err
	}

	return p.rf.NewResMapFromBytes(output)
}
