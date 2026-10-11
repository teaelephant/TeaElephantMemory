GO ?= go
BIN_DIR ?= $(CURDIR)/bin
GOLANGCI_LINT_VERSION := v2.14.0
SQLC_VERSION := v1.31.1

.PHONY: tools generate test lint

tools:
	GOBIN="$(abspath $(BIN_DIR))" $(GO) install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)
	GOBIN="$(abspath $(BIN_DIR))" $(GO) install github.com/sqlc-dev/sqlc/cmd/sqlc@$(SQLC_VERSION)

generate:
	$(GO) generate ./...
	"$(BIN_DIR)/sqlc" generate -f db/sqlc.yaml

test:
	$(GO) test ./...

lint:
	"$(BIN_DIR)/golangci-lint" run
