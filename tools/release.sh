#!/bin/bash

# Get the highest tag number
VERSION="$(git describe --abbrev=0 --tags)"
VERSION=${VERSION:-'0.0.0'}

# Get number parts
MAJOR="${VERSION%%.*}"
VERSION="${VERSION#*.}"
MINOR="${VERSION%%.*}"
VERSION="${VERSION#*.}"
PATCH="${VERSION%%.*}"
VERSION="${VERSION#*.}"

# Increase version
PATCH=$((PATCH + 1))

TAG="${1}"

if [ "${TAG}" = "" ]; then
  TAG="${MAJOR}.${MINOR}.${PATCH}"
fi

echo "Releasing ${TAG} ..."

git-chglog --next-tag="${TAG}" --output CHANGELOG.md
git commit -a -m "Update CHANGELOG for ${TAG}"
git tag -a -s -m "Release ${TAG}" "${TAG}"
git push --tags
goreleaser release \
  --rm-dist \
  --release-notes <(git-chglog --next-tag "${TAG}" "${TAG}" | tail -n+5)
