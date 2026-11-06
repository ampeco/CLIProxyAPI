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

### Patch 5 — retry a failed model-catalog fetch every 30 s

`internal/registry/model_updater.go` (`runModelCatalogUpdater`), also used by the Codex and Devin catalog updaters. Backport of the open upstream PR [#6236](https://github.com/router-for-me/CLIProxyAPI/pull/6236).

Upstream fetches the catalog once at startup and then every 3 h. When the proxy starts before DNS works, the fetch fails and models newer than the embedded catalog answer `400 unknown provider for model` for 3 h. After a failed attempt, retry in 30 s; after any successful fetch, go back to 3 h. Drop this patch when upstream merges #6236 or an equivalent.

### Patch 6 — register models of loaded auth files before the listener opens

`sdk/cliproxy/service_lifecycle.go` (`syncPluginModelRuntime` before `server.Start`). Backport of the open upstream PR [#6464](https://github.com/router-for-me/CLIProxyAPI/pull/6464); upstream closed the issue, [#6702](https://github.com/router-for-me/CLIProxyAPI/issues/6702), as not planned.

Without it, requests in the first second after a restart get `400 unknown provider` for OAuth-file models, because those models are registered only after the file watcher's first load.

### Patch 7 — wait up to 10 s for the first catalog fetch before listening

`internal/registry/model_updater.go` (`WaitForStartupModelRefresh`) and `sdk/cliproxy/service_lifecycle.go`. Not upstream; [#6257](https://github.com/router-for-me/CLIProxyAPI/pull/6257) (open) solves the same race with a 503 gate.

Patch 6 registers models from the catalog in memory at that moment. Right after a restart that is the embedded catalog, so models added upstream since the build still fail until the background fetch lands. The start waits for the first fetch (success or failure) and gives up after 10 s.

## Tests

```bash
go test ./internal/translator/openai/claude/...   # Patches 1 + 2
go test ./internal/registry/...                    # Patches 4, 5, 7
go test ./sdk/cliproxy/ -run TestServiceRun_RegistersLoadedAuthModelsBeforeListening   # Patch 6
```

## Rebase

Monthly intentional bump — no auto-pull. When rebasing:

1. `git fetch upstream`
2. Cherry-pick or rebase the patches on top of the new upstream tag.
3. `go test ./...` — the patch tests must pass.
4. Cut a release tag `vX.Y.Z-ampeco` with a `darwin-arm64` binary + `.sha256` sidecar attached.

## License

MIT — see [LICENSE](LICENSE).
