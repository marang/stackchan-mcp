# Internals

## Binary Modes

`stackchan-mcp` has three primary runtime modes:

- `setup` stores credentials in the desktop Secret Service.
- `serve` runs the MCP stdio server. Codex uses this mode directly.
- `bridge` connects to the XiaoZhi WebSocket endpoint and forwards MCP traffic
  to a local `serve` process.

It also includes CLI helpers:

- `xiaozhi-store-url`
- `linear-store-api-key`
- `resolve`
- `start`
- `finish`

When called without arguments, the binary uses a practical default:

- if stdin is a terminal, it starts `bridge`
- if stdin is piped, it starts `serve`

## Source Layout

```text
cmd/stackchan-mcp/      binary entry point
internal/app/           CLI, XiaoZhi bridge, and MCP server orchestration
internal/issuework/     Linear ticket worktree and tmux orchestration
internal/linearclient/  Linear GraphQL client
internal/search/        web search, page scraping, and URL safety checks
internal/secretstore/   Secret Service wrapper around secret-tool
```

## Development

```bash
make test
make fmt
make build
make clean
```
