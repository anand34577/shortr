#!/usr/bin/env bash
# Copies docs/wiki/*.md into a checkout of the GitHub wiki repo, rewriting
# links so they resolve on the wiki (no .md suffix, absolute repo links).
# Usage: scripts/sync-wiki.sh <path-to-wiki-checkout>
set -euo pipefail

dest="${1:?usage: sync-wiki.sh <wiki-checkout>}"
repo="${GITHUB_REPOSITORY:-anand34577/shortr}"
src="$(cd "$(dirname "$0")/../docs/wiki" && pwd)"
blob="https://github.com/${repo}/blob/main"

for f in "$src"/*.md; do
  sed -E \
    -e "s#\]\(\.\./\.\./\.\./\.\./releases\)#](https://github.com/${repo}/releases)#g" \
    -e "s#\]\(\.\./\.\./([^)]*)\)#](${blob}/\1)#g" \
    -e 's#\]\(([A-Za-z-]+)\.md(\#[^)]*)?\)#](\1\2)#g' \
    "$f" > "$dest/$(basename "$f")"
done

cat > "$dest/_Sidebar.md" <<'SIDE'
**Shortr**

- [Home](Home)
- [Installation](Installation)
- [Configuration](Configuration)
- [API Reference](API-Reference)
- [MCP Server](MCP-Server)
- [Deployment](Deployment)
- [Backup and Restore](Backup-and-Restore)
- [Security](Security)
- [Troubleshooting](Troubleshooting)
- [Contributing](Contributing)
SIDE
