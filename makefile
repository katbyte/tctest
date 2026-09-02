# recipes use bash for pipefail support (ubuntu's default sh is dash)
SHELL := /bin/bash

GIT_COMMIT=$(shell git describe --always --long --dirty)
GIT_VERSION=$(shell git describe --tags --dirty 2>/dev/null | sed 's/-\([0-9]*\)-g/+\1@g/' || echo dev)
TEST_TIMEOUT?=15m

# dev tool binaries are built into .tools/bin (gitignored) from the versions pinned in
# .tools/go.mod - the single source of truth for make and CI; dependabot keeps them updated
TOOLS_BIN=.tools/bin
ACTIONLINT=$(TOOLS_BIN)/actionlint
GOFUMPT=$(TOOLS_BIN)/gofumpt
GOLANGCI_LINT=$(TOOLS_BIN)/golangci-lint

$(ACTIONLINT): .tools/go.mod .tools/go.sum
	@echo "==> building actionlint (version pinned in .tools/go.mod)..."
	@cd .tools && go build -o bin/actionlint github.com/rhysd/actionlint/cmd/actionlint

$(GOFUMPT): .tools/go.mod .tools/go.sum
	@echo "==> building gofumpt (version pinned in .tools/go.mod)..."
	@cd .tools && go build -o bin/gofumpt mvdan.cc/gofumpt

$(GOLANGCI_LINT): .tools/go.mod .tools/go.sum
	@echo "==> building golangci-lint (version pinned in .tools/go.mod)..."
	@cd .tools && go build -o bin/golangci-lint github.com/golangci/golangci-lint/v2/cmd/golangci-lint

default: fmt build

all: fmt build

help: ## Show this help
	@awk 'BEGIN {FS = ":.*##"; printf "Usage: make \033[36m<target>\033[0m\n"} /^[a-zA-Z0-9_-]+:.*?##/ { printf "  \033[36m%-24s\033[0m%s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) }' $(MAKEFILE_LIST)

##@ Build
build: ## Compile tctest with version info from git
	@echo "==> building..."
	go build -ldflags "-X github.com/katbyte/tctest/lib/version.GitCommit=${GIT_COMMIT} -X github.com/katbyte/tctest/lib/version.Version=${GIT_VERSION}"

install: ## Install tctest into GOPATH/bin with version info from git
	@echo "==> installing..."
	go install -ldflags "-X github.com/katbyte/tctest/lib/version.GitCommit=${GIT_COMMIT} -X github.com/katbyte/tctest/lib/version.Version=${GIT_VERSION}" .

tools: $(ACTIONLINT) $(GOFUMPT) $(GOLANGCI_LINT) ## Build the pinned dev tools from .tools/go.mod into .tools/bin (shellcheck and yamllint come from brew/apt)

##@ Formatting
fmt: $(GOFUMPT) $(GOLANGCI_LINT) ## Fix Go formatting (gofmt, gofumpt, goimports)
	@echo "==> Fixing source code with gofmt..."
	find . -name '*.go' | grep -v vendor | xargs gofmt -s -w
	@echo "==> Fixing source code with gofumpt..."
	find . -name '*.go' | grep -v vendor | xargs $(GOFUMPT) -w
	@echo "==> Fixing imports with golangci-lint (goimports)..."
	$(GOLANGCI_LINT) fmt -E goimports ./...

goimports: $(GOLANGCI_LINT) ## Fix imports with golangci-lint (goimports)
	@echo "==> Fixing imports with golangci-lint (goimports)..."
	$(GOLANGCI_LINT) fmt -E goimports ./...

##@ Linting & Dependencies
lint: $(GOLANGCI_LINT) ## Check source code with the golangci linters
	@echo "==> Checking source code against linters..."
	$(GOLANGCI_LINT) run ./...

actionlint: $(ACTIONLINT) ## Check GitHub workflows with actionlint (shellcheck rule is skipped if shellcheck is not installed)
	@echo "==> Checking workflows with actionlint..."
	@$(ACTIONLINT) -shellcheck=shellcheck

lint-fix: $(GOLANGCI_LINT) ## Fix source code with all golangci linters
	@echo "==> Checking source code against linters (applying autofixes)..."
	$(GOLANGCI_LINT) run --fix ./...

yamllint: ## Check YAML files with yamllint (config in .yamllint.yml)
	@command -v yamllint >/dev/null || (echo "yamllint not installed. Install via: brew install yamllint (macOS) or apt/pip install yamllint (Linux)" && exit 1)
	@echo "==> Checking YAML files with yamllint..."
	@yamllint -s .

shellcheck: ## Check shell scripts with shellcheck
	@command -v shellcheck >/dev/null || (echo "shellcheck not installed. Install via: brew install shellcheck (macOS) or apt install shellcheck (Linux)" && exit 1)
	@echo "==> Checking shell scripts with shellcheck..."
	@shellcheck scripts/*.sh .github/images/*.sh

depscheck: ## Check that go.mod/go.sum and vendor/ are in sync
	@echo "==> Checking source code with go mod tidy..."
	@go mod tidy
	@git diff --exit-code -- go.mod go.sum || \
		(echo; echo "Unexpected difference in go.mod/go.sum files. Run 'go mod tidy' command or revert any go.mod/go.sum changes and commit."; exit 1)
	@echo "==> Checking source code with go mod vendor..."
	@go mod vendor
	@git diff --compact-summary --exit-code -- vendor || \
		(echo; echo "Unexpected difference in vendor/ directory. Run 'go mod vendor' command or revert any go.mod/go.sum/vendor changes and commit."; exit 1)
	@echo "==> Checking .tools/go.mod with go mod tidy..."
	@cd .tools && go mod tidy
	@git diff --exit-code -- .tools/go.mod .tools/go.sum || \
		(echo; echo "Unexpected difference in .tools/go.mod/go.sum. Run 'cd .tools && go mod tidy' and commit."; exit 1)

##@ Testing
test: build ## Run unit and integration tests
	go test $$(go list ./... | grep -v '/integration$$') -timeout ${TEST_TIMEOUT}
	@set -o pipefail; go test ./integration/ -v -timeout ${TEST_TIMEOUT} | grep -vE "^=== |--- PASS|^PASS$$"

check-all: build test lint actionlint yamllint shellcheck depscheck ## Run build + test + all linters + depscheck

.PHONY: default all help fmt goimports build lint lint-fix actionlint yamllint shellcheck depscheck check-all install tools test
