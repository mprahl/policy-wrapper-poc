
.PHONY: build build-kustomize build-native generate layout

# Kustomize plugin configuration
XDG_CONFIG_HOME ?= ${HOME}/.config
KUSTOMIZE_PLUGIN_HOME ?= $(XDG_CONFIG_HOME)/kustomize/plugin
API_PLUGIN_PATH ?= $(KUSTOMIZE_PLUGIN_HOME)/policy.open-cluster-management.io/v1/policygenerator

# Native kustomize Go plugin
KUSTOMIZE_PATH ?= $(shell which kustomize)
KUSTOMIZE_CUSTOM_PATH ?= $(shell go env GOPATH)/bin/kustomize

build: layout
	go build -o $(API_PLUGIN_PATH)/PolicyGenerator cmd/exec/main.go

build-native: layout
	go build -buildmode plugin -o $(API_PLUGIN_PATH)/PolicyGenerator.so cmd/native/main.go

generate:
	@SOURCE_DIR=$${SOURCE_DIR:-"examples/exec/"}; \
	KUSTOMIZE_PLUGIN_HOME=$(KUSTOMIZE_PLUGIN_HOME) kustomize build --enable-alpha-plugins $${SOURCE_DIR}

generate-native:
	@SOURCE_DIR=$${SOURCE_DIR:-"examples/native/"}; \
	@KUSTOMIZE_PLUGIN_HOME=$(KUSTOMIZE_PLUGIN_HOME) $(KUSTOMIZE_CUSTOM_PATH) build --enable-alpha-plugins $${SOURCE_DIR}

layout:
	mkdir -p $(API_PLUGIN_PATH)
