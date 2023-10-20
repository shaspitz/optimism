SHELL := /usr/bin/env bash

pkg := bindings
monorepo-base := $(shell dirname $(realpath .))
contracts-dir := $(monorepo-base)/packages/contracts-bedrock
ETHERSCAN_APIKEY ?=

all: version mkdir bindings

version:
	forge --version
	abigen --version

compile:
	cd $(contracts-dir) && \
		forge clean && \
		pnpm build

bindings: compile bindings-build

bindings-build:
	go run ./gen/main.go \
		-forge-artifacts $(contracts-dir)/forge-artifacts \
		-out ./bindings \
		-contracts ./artifacts.json \
		-source-maps MIPS,PreimageOracle \
		-package $(pkg) \
		-monorepo-base $(monorepo-base)

bindgen: compile bindgen-generate-all

bindgen-local: compile bindgen-generate-local

bindgen-etherscan: compile bindgen-generate-etherscan

bindgen-generate-all:
	go run ./bindgen/ \
		generate \
		--metadata-out ./$(pkg) \
		--go-package $(pkg) \
		--monorepo-base $(monorepo-base) \
		all \
		--local-contracts ./artifacts.json \
		--source-maps-list MIPS,PreimageOracle \
		--forge-artifacts $(contracts-dir)/forge-artifacts \
		--etherscan-contracts ./artifacts.json \
		--etherscan-apikey $(ETHERSCAN_APIKEY)

bindgen-generate-local:
	go run ./bindgen/ \
		generate \
		--metadata-out ./$(pkg) \
		--go-package $(pkg) \
		--monorepo-base $(monorepo-base) \
		local \
		--local-contracts ./artifacts.json \
		--source-maps-list MIPS,PreimageOracle \
		--forge-artifacts $(contracts-dir)/forge-artifacts

bindgen-generate-etherscan:
	go run ./bindgen/ \
		generate \
		--metadata-out ./$(pkg) \
		--go-package $(pkg) \
		--monorepo-base $(monorepo-base) \
		etherscan \
		--etherscan-contracts ./artifacts.json \
		--etherscan-apikey $(ETHERSCAN_APIKEY)

lint:
	golangci-lint run -E goimports,sqlclosecheck,bodyclose,asciicheck,misspell,errorlint --timeout 5m -e "errors.As" -e "errors.Is" ./...

mkdir:
	mkdir -p $(pkg)

clean-contracts:
	cd $(contracts-dir) && \
		pnpm clean

clean:
	rm -rf $(pkg)

test:
	go test ./...
