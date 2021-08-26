module github.com/mprahl/go-playground

go 1.16

replace github.com/open-cluster-management/go-template-utils => /home/mprahl/git/go-template-utils

require (
	github.com/imdario/mergo v0.3.12 // indirect
	gopkg.in/yaml.v3 v3.0.0-20210107192922-496545a6307b
	k8s.io/apimachinery v0.22.1
	sigs.k8s.io/kustomize/api v0.9.0
	sigs.k8s.io/kustomize/kyaml v0.11.1
)
