#!/usr/bin/env bash
# Run the docs site locally in Docker, with live reload:
#
#   scripts/docs-serve.sh          # http://localhost:8000
#
# The Dockerfile pins the same mkdocs-material version CI builds with
# (.github/workflows/docs.yml), so what you see on :8000 is what the site
# deploys. The working tree is mounted over the image's copy, so edits to
# docs/ reload without a rebuild.
set -euo pipefail

cd "$(dirname "$0")/.."

image=spinloop-docs

docker build -q -f scripts/docs.Dockerfile -t "$image" .

echo "Docs: http://localhost:8000"
exec docker run --rm -p 8000:8000 -v "$(pwd)":/src "$image" serve -a 0.0.0.0:8000
