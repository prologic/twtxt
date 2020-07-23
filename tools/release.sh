#!/bin/sh

set -e

# shellcheck source=./tools/functions.sh
. "$(dirname "$0")/functions.sh"

TAG="${TAG}"

generate_next_tag() {
  progress "Generating next tag"

  if [ -z "$TAG" ]; then
    version="$(git describe --abbrev=0 --tags)"
    TAG="$(bump_version "$version")"
  fi
}

generate_changelog() {
  progress "Generating chnagelog for $TAG"
  (
    git-chglog --next-tag="${TAG}" --output CHANGELOG.md
    git ci -a -m "Release ${TAG}"
    git push -q
  ) >&2
}

create_draft_release() {
  progress "Creating draft release for $TAG"
  (
    github-release release \
      -u prologic \
      -r twtxt \
      -t "${TAG}" \
      -n "${TAG}" \
      -d "$(git-chglog --next-tag "${TAG}" "${TAG}" | tail -n+5)" \
      --draft
  ) >&2
}

steps="generate_next_tag generate_changelog create_draft_release"

_main() {
  for step in $steps; do
    if ! run "$step"; then
      fail "Release failed"
    fi
  done

  echo "🎉 All Done!"
}

if [ -n "$0" ] && [ x"$0" != x"-bash" ]; then
  if ! _main "$@"; then
    fail "Release failed"
  fi
fi
