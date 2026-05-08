.PHONY: help all build install test fmt setup start run debug serve clean FORCE

DIST_DIR := dist
BINARY := $(DIST_DIR)/stackchan-mcp

help:
	@printf '%s\n' 'StackChan MCP targets:'
	@printf '  %-12s %s\n' 'make build' 'Build the single binary: dist/stackchan-mcp'
	@printf '  %-12s %s\n' 'make install' 'Install stackchan-mcp with go install'
	@printf '  %-12s %s\n' 'make start' 'Start StackChan/XiaoZhi bridge; it runs the same binary as MCP serve in the background'
	@printf '  %-12s %s\n' 'make debug' 'Start bridge with JSON-RPC debug logs'
	@printf '  %-12s %s\n' 'make serve' 'Run MCP stdio server mode for Codex or manual checks'
	@printf '  %-12s %s\n' 'make setup' 'Store XiaoZhi URL and Linear API key in Secret Service'
	@printf '  %-12s %s\n' 'make test' 'Run Go tests'
	@printf '  %-12s %s\n' 'make clean' 'Remove dist/ and old root binaries'
	@printf '%s\n' ''
	@printf '%s\n' 'Binary modes:'
	@printf '  %-34s %s\n' './dist/stackchan-mcp bridge' 'StackChan/XiaoZhi runtime'
	@printf '  %-34s %s\n' './dist/stackchan-mcp serve' 'MCP stdio server'
	@printf '  %-34s %s\n' './dist/stackchan-mcp setup' 'Secret setup'

all: test build

build: $(BINARY)

install:
	go install ./cmd/stackchan-mcp

$(DIST_DIR):
	mkdir -p $(DIST_DIR)

$(DIST_DIR)/stackchan-mcp: FORCE | $(DIST_DIR)
	go build -o $@ ./cmd/stackchan-mcp

test:
	go test ./...

fmt:
	gofmt -w cmd/stackchan-mcp/*.go internal/app/*.go internal/issuework/*.go internal/linearclient/*.go internal/search/*.go internal/secretstore/*.go

setup: $(BINARY)
	./$(BINARY) setup

start: $(BINARY)
	./$(BINARY) bridge

run: start

debug: $(BINARY)
	./$(BINARY) bridge --debug

serve: $(BINARY)
	./$(BINARY) serve

clean:
	rm -rf $(DIST_DIR)
	rm -f stackchan-mcp issue-work xiaozhi-bridge

FORCE:
