.PHONY: build build-kustomize build-native generate layout

KUSTOMIZE_PATH ?= $(shell which kustomize)
KUSTOMIZE_CUSTOM_PATH := $(shell go env GOPATH)/bin/kustomize
KUSTOMIZE_PLUGIN_DIR := kustomize/plugin/policy.open-cluster-management.io/v1/policygenerator

build: layout
	go build -o $(KUSTOMIZE_PLUGIN_DIR)/PolicyGenerator cmd/exec/main.go

build-native: layout
	go build -buildmode plugin -o $(KUSTOMIZE_PLUGIN_DIR)/PolicyGenerator.so cmd/native/main.go

generate:
	@XDG_CONFIG_HOME=./ ${KUSTOMIZE_PATH} build --enable-alpha-plugins examples/exec

generate-native:
	@XDG_CONFIG_HOME=./ ${KUSTOMIZE_CUSTOM_PATH} build --enable-alpha-plugins examples/native

layout:
	mkdir -p $(KUSTOMIZE_PLUGIN_DIR)
