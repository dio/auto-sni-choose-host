MODULE_NAME := auto_sni_choose_host
OUTPUT := .bin/lib$(MODULE_NAME).so

UNAME_S := $(shell uname -s)
UNAME_M := $(shell uname -m)

ifeq ($(UNAME_S),Darwin)
ENVOY_OS := darwin
DEFAULT_TRUSTED_CA := /etc/ssl/cert.pem
else
ENVOY_OS := linux
DEFAULT_TRUSTED_CA := /etc/ssl/certs/ca-certificates.crt
endif

ifeq ($(UNAME_M),arm64)
ENVOY_ARCH := arm64
else ifeq ($(UNAME_M),aarch64)
ENVOY_ARCH := arm64
else
ENVOY_ARCH := amd64
endif

ENVOY_BUILDER_REPO ?= dio/envoy-builder
ENVOY_RELEASE_TAG ?= envoy-0d6e3c60-auto-host-sni-bounded-sni-session-cache
ENVOY_PATCH_FLAVOR ?= auto-host-sni-bounded-sni-session-cache
ENVOY_ASSET ?= envoy-$(ENVOY_OS)-$(ENVOY_ARCH)-$(ENVOY_PATCH_FLAVOR)
ENVOY_BIN ?= .bin/envoy
TRUSTED_CA ?= $(DEFAULT_TRUSTED_CA)

.PHONY: build clean config download-envoy run test request-example request-iana

build:
	mkdir -p .bin
	CGO_ENABLED=1 go build -buildmode=c-shared -o $(OUTPUT) ./cmd/module

config:
	mkdir -p .bin
	sed 's|@TRUSTED_CA@|$(TRUSTED_CA)|g' config/envoy.yaml.in > .bin/envoy.yaml

download-envoy:
	mkdir -p .bin
	@if [ ! -x "$(ENVOY_BIN)" ]; then \
		echo "downloading patched Envoy $(ENVOY_ASSET) from $(ENVOY_BUILDER_REPO)/$(ENVOY_RELEASE_TAG)"; \
		curl -fsSL -L \
			"https://github.com/$(ENVOY_BUILDER_REPO)/releases/download/$(ENVOY_RELEASE_TAG)/$(ENVOY_ASSET)" \
			-o "$(ENVOY_BIN)"; \
		chmod +x "$(ENVOY_BIN)"; \
	fi

run: build config download-envoy
	ENVOY_DYNAMIC_MODULES_SEARCH_PATH="$(CURDIR)/.bin" \
	GODEBUG=cgocheck=0 \
	"$(ENVOY_BIN)" -c .bin/envoy.yaml --component-log-level dynamic_modules:info

test:
	go test ./choosehost

request-example:
	curl -v -H 'x-target-host: example.com' http://127.0.0.1:10000/

request-iana:
	curl -v -H 'x-target-host: www.iana.org' http://127.0.0.1:10000/domains/reserved

clean:
	rm -rf .bin
