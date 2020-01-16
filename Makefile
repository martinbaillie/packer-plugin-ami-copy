SHELL 		:=$(shell which bash)
.SHELLFLAGS =-c

ifndef DEBUG
.SILENT: ;
endif
.EXPORT_ALL_VARIABLES: ;

WORKDIR 	=$(patsubst %/,%,$(dir $(realpath $(lastword $(MAKEFILE_LIST)))))
PROJECT 	=$(notdir $(WORKDIR))

DIR 	:=$(shell pwd)
TIME 	:=$(shell date '+%Y-%m-%dT%T%z')
GPG_KEY ?=$(shell git config user.signingkey)

SRC 		?=$(shell go list ./...)
SRCFILES	?=$(shell find . -type f -name '*.go')

LDFLAGS ?=-s -w -extld ld -extldflags -static
FLAGS	?=-a -installsuffix cgo -ldflags "$(LDFLAGS)"

GOVERS 		=$(shell go version)
GOOS		=$(word 1,$(subst /, ,$(lastword $(GOVERS))))
GOARCH		=$(word 2,$(subst /, ,$(lastword $(GOVERS))))
GOOSES		=darwin linux windows
GOARCHES 	=amd64

.DEFAULT_GOAL := build

.PHONY: clean fix fmt generate test build tag help

clean: ## Removes binary and generated files
	echo >&2 ">> cleaning"
	rm -rf $(PROJECT)*

fix: ## Runs the Golang fix tool
	echo >&2 ">> fixing"
	go fix $(SRC)

fmt: ## Formats source code according to the Go standard
	echo >&2 ">> formatting"
	gofmt -w -s -l $(SRCFILES)

generate: ## Runs the Golang generate tool
	echo >&2 ">> generating"
	go install github.com/hashicorp/packer/cmd/mapstructure-to-hcl2
	go generate ./...

test: clean fix fmt generate

# Create a cross-compile target for every os/arch pairing. This will generate a
# non-phony make target for each os/arch pair as well as a phony meta target
# (build) for compiling everything.
_build:
	echo >&2 ">> building"
.PHONY: _build
define build-target
  $(PROJECT)-$(1)-$(2)$(3):
  ifeq (,$(findstring $(1)-$(2),$(NOARCHES)))
		echo >&2 ">>>> $$@"
		env GOOS=$(1) GOARCH=$(2) CGO_ENABLED=0 go build $(FLAGS) -o $$@
  endif

  build: _build $(PROJECT)-$(1)-$(2)$(3)
endef
$(foreach goarch,$(GOARCHES), \
	$(foreach goos,$(GOOSES), \
		$(eval \
			$(call build-target,$(goos),$(goarch),$(if \
				$(findstring windows,$(goos)),.exe,)\
			) \
		) \
	) \
)
build: ## Build for every supported OS and arch combination
.PHONY: build

tag: ## Create a signed commit and tag
	echo >&2 ">> tagging"
	if [[ ! $(VERSION) =~ ^[0-9]+[.][0-9]+([.][0.9]*)?$  ]]; then \
		echo >&2 "ERROR: VERSION ($(VERSION)) is not a semantic version"; \
		exit 1; \
	fi
	echo >&2 ">>>> v$(VERSION)"
	git commit \
		--allow-empty \
		--gpg-sign="$(GPG_KEY)" \
		--message "Release v$(VERSION)" \
		--quiet \
		--signoff
	git tag \
		--annotate \
		--create-reflog \
		--local-user "$(GPG_KEY)" \
		--message "Version $(VERSION)" \
		--sign \
		"v$(VERSION)" master
.PHONY: tag

# A help target including self-documenting targets (see the awk statement)
help: FORMAT="\033[36m%-30s\033[0m %s\n"
help: ## This help target
	awk 'BEGIN {FS = ":.*?## "} /^[%a-zA-Z_-]+:.*?## / \
		{printf $(FORMAT), $$1, $$2}' $(MAKEFILE_LIST)
	printf $(FORMAT) $(PROJECT)-%-% \
		"Build for a specific OS and arch (where '%' = OS, arch)"
.PHONY: help
