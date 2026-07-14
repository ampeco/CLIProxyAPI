#!/usr/bin/env bash
set -euo pipefail

models_repository="${MODELS_REPOSITORY_URL:-https://github.com/router-for-me/models.git}"
models_ref="${MODELS_REPOSITORY_REF:-main}"
catalog_dir="${MODEL_CATALOG_DIR:-internal/registry/models}"
codex_catalog="$catalog_dir/codex_client_models.json"
codex_candidate="$(mktemp)"
trap 'rm -f "$codex_candidate"' EXIT

git fetch --depth 1 "$models_repository" "$models_ref"
git show FETCH_HEAD:models.json > "$catalog_dir/models.json"

if git show FETCH_HEAD:codex_client_models.json > "$codex_candidate" &&
  go run ./cmd/validate_codex_models --file "$codex_candidate"; then
	mv "$codex_candidate" "$codex_catalog"
	printf 'Refreshed validated Codex client model catalog.\n'
else
	printf '::warning::Remote Codex client model catalog is missing or invalid; using embedded fallback.\n'
fi

go run ./cmd/validate_codex_models --file "$codex_catalog"

# --- ampeco Patch 4: strip Claude entries from the antigravity catalog ---
# Antigravity's published catalog advertises Claude models served by Google's
# cloudcode-pa.googleapis.com. With those entries present, an antigravity OAuth
# becomes eligible alongside the Anthropic OAuth pool for `claude-*` selector
# picks; session-affinity then pins entire Claude Code sessions to the
# antigravity OAuth, exhausting its small Sonnet quota within hours. Strip them
# post-refresh so the antigravity OAuth is Gemini-only.
python3 - <<'PY'
import json
p = "internal/registry/models/models.json"
with open(p) as f:
    d = json.load(f)
if "antigravity" in d:
    before = len(d["antigravity"])
    d["antigravity"] = [
        m for m in d["antigravity"]
        if not (
            m.get("id", "").startswith("claude-")
            or m.get("type", "") == "claude"
            or m.get("owned_by", "") == "anthropic"
        )
    ]
    after = len(d["antigravity"])
    print(f"antigravity catalog: {before} -> {after} entries ({before - after} claude stripped)")
with open(p, "w") as f:
    json.dump(d, f, indent=2)
    f.write("\n")
PY
