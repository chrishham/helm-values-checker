#!/usr/bin/env bash
set -euo pipefail

VERSION="${1:-}"

if [[ -z "$VERSION" ]]; then
  echo "Usage: $0 <version>"
  echo "Example: $0 0.3.0"
  exit 1
fi

# Strip leading 'v' if provided
VERSION="${VERSION#v}"
TAG="v${VERSION}"

# Ensure we're on main and clean
BRANCH=$(git rev-parse --abbrev-ref HEAD)
if [[ "$BRANCH" != "main" ]]; then
  echo "Error: must be on main branch (currently on ${BRANCH})"
  exit 1
fi

if ! git diff --quiet || ! git diff --cached --quiet; then
  echo "Error: working tree is not clean. Commit or stash changes first."
  exit 1
fi

# Check tag doesn't already exist
if git rev-parse "$TAG" >/dev/null 2>&1; then
  echo "Error: tag ${TAG} already exists"
  exit 1
fi

# Bump version in plugin.yaml
sed -i "s/^version: .*/version: \"${VERSION}\"/" plugin.yaml

echo "Bumped plugin.yaml to ${VERSION}"

git add plugin.yaml
git commit -m "Bump version to ${VERSION}"
git tag "$TAG"
git push origin main "$TAG"

echo "Pushed ${TAG} — release workflow will run automatically."
