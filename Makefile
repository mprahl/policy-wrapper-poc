build:
	go build
	mkdir -p kustomize/plugin/policy.open-cluster-management.io/v1/policygenerator/
	mv PolicyGenerator kustomize/plugin/policy.open-cluster-management.io/v1/policygenerator

generate:
	@XDG_CONFIG_HOME=./ kustomize build --enable-alpha-plugins