# Variables
GO ?= go
GO_PACKAGES=$(shell $(GO) list ./... | grep -v vendor)

# Default target
.PHONY: all
all: help

# Help target
.PHONY: help
help:
	@echo "Available targets:"
	@echo "  vet         Run go vet on all packages"
	@echo "  lint        Run golangci-lint on all packages"
	@echo "  test        Run go test on all packages"
	@echo "  deps        Download go module dependencies"
	@echo "  install-lint  Install golangci-lint if not present"
	@echo "  clean       Remove build artifacts (if any)"

# Dependency installation
.PHONY: deps
deps:
	$(GO) mod tidy

# Install golangci-lint if not present
.PHONY: install-lint
install-lint:
	@if ! [ -x "$(GOBIN)/golangci-lint" ]; then \
	  echo "Installing golangci-lint..."; \
	  curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(GOBIN) v2.1.6; \
	else \
	  echo "golangci-lint already installed"; \
	fi

# Get default value of $GOBIN if not explicitly set
GO_PATH=$(shell go env GOPATH)
ifeq (,$(shell go env GOBIN))
  GOBIN=${GO_PATH}/bin
else
  GOBIN=$(shell go env GOBIN)
endif

vet:
	go vet ${GO_PACKAGES}

# Run configured linters
lint:
	golangci-lint run --timeout 10m0s

# Test target
.PHONY: test
test:
	$(GO) test -v ${GO_PACKAGES}

# Clean target
.PHONY: clean
clean:
	@echo "Nothing to clean (add commands if needed)"
