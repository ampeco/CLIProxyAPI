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

### Patch 3 — RETIRED (upstream since v7.2.140, deployment uses OAuth files)

Previously: OAuth access tokens (`sk-ant-oat01-*`) stored in `claude-api-key:` entries were routed via `Authorization: Bearer …` instead of `x-api-key`. Retired in the v7.2.140 sync because (a) upstream now detects OAuth tokens natively (`isClaudeOAuthToken` in `internal/runtime/executor/claude_executor_request.go` forces Bearer on the request path), and (b) the AMPECO deployment migrated from `claude-api-key:` token entries to OAuth credential files (`--claude-login`), so the api-key code path no longer carries OAuth tokens at all.

### Patch 4 — strip Claude entries from the `antigravity` model catalog

`internal/registry/models/models.json` and `.github/scripts/refresh-model-catalogs.sh` (post-refresh strip; the script is invoked by every build job in `release.yaml`).

Antigravity's published catalog includes `claude-opus-4-6-thinking` and `claude-sonnet-4-6` because Google's AI Code Assist tier resells Claude models via `cloudcode-pa.googleapis.com`. With those entries present, an antigravity OAuth becomes eligible alongside the Anthropic OAuth pool when the selector picks for `claude-*` routes. Session-affinity then pins entire Claude Code sessions (parent + sub-agents) to the antigravity OAuth whenever the parent request was a Gemini call, exhausting Antigravity's small Sonnet quota within hours.

Strip every entry whose `id` starts with `claude-`, whose `type` is `claude`, or whose `owned_by` is `anthropic` from the `antigravity` array. Applied both to the in-tree `models.json` (so `go test ./...` and local dev see the patched shape) and to `.github/scripts/refresh-model-catalogs.sh` (so upstream catalog refreshes during tagged builds re-apply the strip automatically). After the patch the antigravity OAuth is Gemini-only and Claude requests fall through to the Anthropic OAuth pool.

## Tests

```bash
go test ./internal/translator/openai/claude/...   # Patches 1 + 2
go test ./internal/registry/...                    # Patch 4 (catalog integrity)
```

## Rebase

Monthly intentional bump — no auto-pull. When rebasing:

1. `git fetch upstream`
2. Cherry-pick or rebase the patches on top of the new upstream tag.
3. `go test ./...` — the patch tests must pass.
4. Cut a release tag `vX.Y.Z-ampeco` with a `darwin-arm64` binary + `.sha256` sidecar attached.

## License

MIT — see [LICENSE](LICENSE).
