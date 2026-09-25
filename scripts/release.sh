#!/usr/bin/env bash
# Bumps VERSION per semver, commits just that file, tags the commit
# v<new-version>, and pushes both the branch and the tag — triggering
# .github/workflows/release.yml. Invoked via `make release`/`release-minor`/
# `release-major`; not intended to be run directly by hand, but safe to.
#
# This is the one deliberate exception to lazytask's "never auto-commit"
# rule: the commit it creates is a single mechanical VERSION-file bump
# tied 1:1 to the release action itself (npm's `npm version` does the
# same). It refuses to run on a dirty tree, so no unrelated changes can
# ever be swept into that commit.
set -euo pipefail

bump_type="${1:-patch}"
case "$bump_type" in
patch | minor | major) ;;
*)
	echo "Usage: $0 [patch|minor|major]" >&2
	exit 1
	;;
esac

if [ -n "$(git status --porcelain)" ]; then
	echo "Working tree not clean; commit or stash changes first." >&2
	exit 1
fi

current="$(cat VERSION)"
if ! [[ "$current" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
	echo "VERSION '$current' isn't a plain MAJOR.MINOR.PATCH; bump it by hand." >&2
	exit 1
fi

IFS=. read -r major minor patch <<<"$current"
case "$bump_type" in
patch) patch=$((patch + 1)) ;;
minor)
	minor=$((minor + 1))
	patch=0
	;;
major)
	major=$((major + 1))
	minor=0
	patch=0
	;;
esac
new_version="${major}.${minor}.${patch}"

if git rev-parse "v${new_version}" >/dev/null 2>&1; then
	echo "Tag v${new_version} already exists." >&2
	exit 1
fi

echo "$new_version" >VERSION
git add VERSION
git commit -m "chore: release v${new_version}"
git tag "v${new_version}"
git push origin HEAD
git push origin "v${new_version}"

echo "Released v${new_version} — check the Release workflow in GitHub Actions."
