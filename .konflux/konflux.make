SHELL := /bin/bash

AUTH_FILE?=$(shell echo ${XDG_RUNTIME_DIR}/containers/auth.json)
TARGET?="operator"

.PHONY: rpm-lock
rpm-lock: rhsm-keys
	@echo $(AUTH_FILE)
	podman run --rm -it \
	-v $(shell pwd):/source:z \
	-v $(AUTH_FILE):/.dockerconfig.json:z \
	--env 'REGISTRY_AUTH_FILE=/.dockerconfig.json' \
	--env 'RHSM_ACTIVATION_KEY=$(RHSM_ACTIVATION_KEY)' \
	--env 'RHSM_ORG=$(RHSM_ORG)' \
	--workdir /source \
	registry.redhat.io/rhel9-4-els/rhel:9.4 ./.konflux/hack/generate-rpm-lock.sh $(TARGET)

.PHONY: rhsm-keys
rhsm-keys:
ifndef RHSM_ACTIVATION_KEY
	$(error environment variable RHSM_ACTIVATION_KEY is required)
endif
ifndef RHSM_ORG
	$(error environment variable RHSM_ORG is required)
endif

.PHONY: konflux-update
konflux-update: konflux-task-manifest-updates

.PHONY: konflux-task-manifest-updates
konflux-task-manifest-updates:
	.konflux/hack/konflux_update_task_refs.sh .tekton/build-pipeline.yaml
