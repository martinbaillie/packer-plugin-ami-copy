SHELL 	:= /bin/bash
DIR 	:=$(shell pwd)
TIME 	:=$(shell date '+%Y-%m-%dT%T%z')

SRC 		?=$(shell go list ./...)
SRCFILES	?=$(shell find . -type f -name '*.go' -not -path './vendor/*')

LDFLAGS ?=-s -w -extld ld -extldflags -static
FLAGS	?=-a -installsuffix cgo -ldflags "$(LDFLAGS)"
GOOS 	?=darwin windows linux
GOARCH 	?=amd64

PROJECT	?=packer-post-processor-ami-copy

.DEFAULT_GOAL := build

.PHONY: dep clean fix fmt generate test build help

dep:: ## Installs build/test dependencies
	@echo ">> dep"
	@go get -u github.com/golang/dep/cmd/dep 
	@dep ensure
# @go get -u github.com/golang/mock/gomock
# @go install github.com/golang/mock/mockgen

clean:: ## Removes binary and generated files
	@echo ">> cleaning"
	@rm -rf $(PROJECT)* mock.go

fix:: ## Runs the Golang fix tool
	@echo ">> fixing"
	@go fix $(SRC)

fmt:: ## Formats source code according to the Go standard
	@echo ">> formatting"
	@gofmt -w -s -l $(SRCFILES)

generate:: ## Runs the Golang generate tool
	@echo ">> generating"
	@go generate ./...

test: clean fix fmt generate

build:: test ## Builds for all arch ($GOARCH) and OS ($GOOS)
	@echo ">> building"
	@for arch in ${GOARCH}; do \
		for os in ${GOOS}; do \
			echo ">>>> $${os}/$${arch}"; \
			env GOOS=$${os} GOARCH=$${arch} \
			CGO_ENABLED=0 go build $(FLAGS) \
			-o $(PROJECT)-$${os}-$${arch}; \
		done; \
	done

# A help target including self-documenting targets (see the awk statement)
help: ## This help target
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / \
		{printf "\033[36m%-30s\033[0m  %s\n", $$1, $$2}' $(MAKEFILE_LIST)
