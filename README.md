# CLI Proxy API — fork

Fork of [router-for-me/CLIProxyAPI](https://github.com/router-for-me/CLIProxyAPI). For project overview, sponsors, ports, and the full feature set, see the [upstream README](https://github.com/router-for-me/CLIProxyAPI/blob/main/README.md).

## Patches

Each patch is strict-additive: it only changes behavior when its condition is met.

### Patch 1 — `reasoning` field fallback in OpenAI→Claude response translation

`internal/translator/openai/claude/openai_claude_response.go` (three call sites).

When `reasoning_content` is absent or empty in the upstream response, fall back to `reasoning`. `reasoning_content` still wins when both are present.

### Patch 2 — strip `cache_control` markers in Claude→OpenAI request translation

`internal/translator/openai/claude/openai_claude_request.go` (`stripCacheControl` helper at the top of `ConvertClaudeRequestToOpenAI`).

Remove `cache_control` from `messages[].content[]`, `system[]`, `tools[]`, and message-level fields before translation.

### Patch 3 — OAuth access tokens use `Authorization: Bearer …` (Claude executor auth)

`internal/runtime/executor/claude_executor.go` (two call sites: `PrepareRequest` and `applyClaudeHeaders`).

When the credential's API key starts with `sk-ant-oat01-`, route via `Authorization: Bearer …` instead of `x-api-key`. Real `sk-ant-api03-*` API keys keep the existing `x-api-key` routing.

## Tests

```bash
go test ./internal/translator/openai/claude/...                                                     # Patches 1 + 2
go test ./internal/runtime/executor/ -run 'OAuthAccessTokenUsesBearerAuth|ApiKeyStillUsesXApiKey'   # Patch 3
```

## Rebase

Monthly intentional bump — no auto-pull. When rebasing:

1. `git fetch upstream`
2. Cherry-pick or rebase the patches on top of the new upstream tag.
3. `go test ./...` — the patch tests must pass.
4. Cut a release tag `vX.Y.Z-ampeco` with a `darwin-arm64` binary + `.sha256` sidecar attached.

## License

MIT — see [LICENSE](LICENSE).
