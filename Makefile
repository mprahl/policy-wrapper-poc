XDG_CONFIG_HOME ?= ${HOME}/.config
KUSTOMIZE_PLUGIN_HOME ?= $(XDG_CONFIG_HOME)/kustomize/plugin
API_PLUGIN_PATH ?= $(KUSTOMIZE_PLUGIN_HOME)/policy.open-cluster-management.io/v1/policygenerator

build:
	go build
	mkdir -p $(API_PLUGIN_PATH)
	cp PolicyGenerator $(API_PLUGIN_PATH)

generate:
	echo $(KUSTOMIZE_PLUGIN_HOME)
	# @KUSTOMIZE_PLUGIN_HOME=$(KUSTOMIZE_PLUGIN_HOME) kustomize build --enable-alpha-plugins
