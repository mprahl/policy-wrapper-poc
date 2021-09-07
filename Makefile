.PHONY: build build-kustomize build-native generate layout

# Kustomize plugin configuration
XDG_CONFIG_HOME ?= ${HOME}/.config
KUSTOMIZE_PLUGIN_HOME ?= $(XDG_CONFIG_HOME)/kustomize/plugin
API_PLUGIN_PATH ?= $(KUSTOMIZE_PLUGIN_HOME)/policy.open-cluster-management.io/v1/policygenerator

# Kustomize arguments
SOURCE_DIR ?= examples/

build: layout
	go build -o $(API_PLUGIN_PATH)/PolicyGenerator cmd/main.go

build-binary:
	go build -o PolicyGenerator cmd/main.go

generate:
	@KUSTOMIZE_PLUGIN_HOME=$(KUSTOMIZE_PLUGIN_HOME) kustomize build --enable-alpha-plugins $(SOURCE_DIR)

layout:
	mkdir -p $(API_PLUGIN_PATH)
