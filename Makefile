.DEFAULT_GOAL := help

GO_PACKAGES ?= ./...
GO_FILES := $(shell find . -type f -name '*.go' -not -path './vendor/*')
PRETTIER_FILES := README.md .golangci.yml .github/workflows/checks.yml .prettierrc.json lefthook.yml
COVERAGE_PROFILE ?= coverage.out
COVERAGE_MIN ?= 80.0

GOLANGCI_LINT ?= go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.2.2
PRETTIER ?= npx --yes prettier@3.5.3

.PHONY: help
help: ## Show available targets
	@awk 'BEGIN { \
		FS = ":.*##"; \
		printf "Usage:\n  make \033[36m<target>\033[0m\n"\
	} \
	/^[^: \t]+:.*?##/ { \
		printf "  \033[36m%-14s\033[0m %s\n", $$1, $$2 \
	} \
	/^##@/ { \
		printf "\n\033[1m%s\033[0m\n", substr($$0, 5) \
	} ' $(MAKEFILE_LIST)

##@ Formatting

.PHONY: fmt
fmt: ## Format Go and repo text files
	gofmt -w $(GO_FILES)
	$(PRETTIER) --write $(PRETTIER_FILES)

.PHONY: fmt-check
fmt-check: ## Verify Go and repo text formatting
	@files="$$(gofmt -l $(GO_FILES))"; \
	if [ -n "$$files" ]; then \
		printf 'gofmt needed:\n%s\n' "$$files"; \
		exit 1; \
	fi
	$(PRETTIER) --check $(PRETTIER_FILES)

##@ Lint

.PHONY: lint
lint: ## Run golangci-lint
	$(GOLANGCI_LINT) run $(GO_PACKAGES)

.PHONY: lint-fix
lint-fix: ## Run golangci-lint with automatic fixes
	$(GOLANGCI_LINT) run --fix $(GO_PACKAGES)

.PHONY: vet
vet: ## Run go vet
	go vet $(GO_PACKAGES)

##@ Test

.PHONY: test
test: ## Run unit tests
	go test $(GO_PACKAGES)

.PHONY: coverage
coverage: ## Run tests with coverage and enforce COVERAGE_MIN
	go test $(GO_PACKAGES) -coverprofile=$(COVERAGE_PROFILE) -covermode=atomic
	@total="$$(go tool cover -func=$(COVERAGE_PROFILE) | awk '/^total:/ { gsub("%", "", $$3); print $$3 }')"; \
	awk -v total="$$total" -v min="$(COVERAGE_MIN)" 'BEGIN { \
		if ((total + 0) < (min + 0)) { \
			printf "coverage %.1f%% is below required %.1f%%\n", total, min; \
			exit 1; \
		} \
		printf "coverage %.1f%% meets required %.1f%%\n", total, min; \
	}'

.PHONY: check
check: fmt-check vet lint coverage ## Run formatting, linting, vet, and coverage
