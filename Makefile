SHELL:=/usr/bin/env bash

BIN_NAME:=ecobee_influx_connector
BIN_VERSION:=$(shell ./.version.sh)

default: help
.PHONY: help  # via https://marmelab.com/blog/2016/02/29/auto-documented-makefile.html
help: ## Print help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'

.PHONY: all
all: clean build-linux-amd64 build-linux-arm64 build-linux-armv7 build-linux-armv6 ## Build for Linux (amd64, arm64, armv7, armv6)

.PHONY: clean
clean: ## Remove build products (./out)
	rm -rf ./out

.PHONY: build
build: ## Build for the current platform & architecture to ./out
	mkdir -p out
	go build -ldflags="-X main.version=${BIN_VERSION}" -o ./out/${BIN_NAME} .

.PHONY: build-linux-amd64
build-linux-amd64: ## Build for Linux/amd64 to ./out
	env GOOS=linux GOARCH=amd64 go build -ldflags="-X main.version=${BIN_VERSION}" -o ./out/${BIN_NAME}-${BIN_VERSION}-linux-amd64 .

.PHONY: build-linux-arm64
build-linux-arm64: ## Build for Linux/arm64 to ./out
	env GOOS=linux GOARCH=arm64 go build -ldflags="-X main.version=${BIN_VERSION}" -o ./out/${BIN_NAME}-${BIN_VERSION}-linux-arm64 .

.PHONY: build-linux-armv7
build-linux-armv7: ## Build for Linux/armv7 to ./out
	env GOOS=linux GOARCH=arm GOARM=7 go build -ldflags="-X main.version=${BIN_VERSION}" -o ./out/${BIN_NAME}-${BIN_VERSION}-linux-armv7 .

.PHONY: build-linux-armv6
build-linux-armv6: ## Build for Linux/armv6 to ./out
	env GOOS=linux GOARCH=arm GOARM=6 go build -ldflags="-X main.version=${BIN_VERSION}" -o ./out/${BIN_NAME}-${BIN_VERSION}-linux-armv6 .

.PHONY: package
package: all ## Build all binaries + .deb packages to ./out (requires fpm: https://fpm.readthedocs.io)
	fpm -t deb -v ${BIN_VERSION} -p ./out/${BIN_NAME}-${BIN_VERSION}-amd64.deb -a amd64 ./out/${BIN_NAME}-${BIN_VERSION}-linux-amd64=/usr/bin/${BIN_NAME}
	fpm -t deb -v ${BIN_VERSION} -p ./out/${BIN_NAME}-${BIN_VERSION}-arm64.deb -a arm64 ./out/${BIN_NAME}-${BIN_VERSION}-linux-arm64=/usr/bin/${BIN_NAME}
	fpm -t deb -v ${BIN_VERSION} -p ./out/${BIN_NAME}-${BIN_VERSION}-armhf.deb -a armhf ./out/${BIN_NAME}-${BIN_VERSION}-linux-armv7=/usr/bin/${BIN_NAME}
	fpm -t deb -v ${BIN_VERSION} -p ./out/${BIN_NAME}-${BIN_VERSION}-armel.deb -a armel ./out/${BIN_NAME}-${BIN_VERSION}-linux-armv6=/usr/bin/${BIN_NAME}

.PHONY: lint
lint: ## Lint all source files in this repository (requires nektos/act: https://nektosact.com)
	act --artifact-server-path /tmp/artifacts -j lint

.PHONY: update-lint
update-lint: ## Pull updated images supporting the lint target (may fetch >10 GB!)
	docker pull catthehacker/ubuntu:full-latest

GOLINT_FILES:=$(shell find . -name '*.go' | grep -v /vendor/)
.PHONY: golint
golint: ## Lint all .go files with golint
	@for file in ${GOLINT_FILES} ;  do \
		echo "$$file" ; \
		golint $$file ; \
	done
