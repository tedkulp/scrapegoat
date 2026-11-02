---
name: release
description: Use when releasing a new version of scrapegoat: checking CI, cutting the CHANGELOG section, tagging, and pushing the tag so GoReleaser publishes. Covers how to verify the release landed.
---

# Releasing scrapegoat

A release is a pushed `vX.Y.Z` tag, and the tag is the only place the version lives: `cmd/scraper/main.go` says `version = "dev"`, and GoReleaser stamps the tag's version in with `-X main.version`. Pushing the tag runs `.github/workflows/release.yml`, which fails before publishing if `CHANGELOG.md` has no `## [X.Y.Z]` section with entries, then runs GoReleaser with that section as the Release notes. GoReleaser builds `scrapegoat` for linux, darwin and windows on amd64 and arm64 and publishes a GitHub Release with six archives and `checksums.txt`.

Work through the steps in order. **A pushed tag publishes**: anyone may fetch the release within minutes, so a re-tag overwrites something people already have.

## 1. Check the starting point

```bash
git switch main && git pull --ff-only
git status --short
gh run list --branch main --workflow ci.yml --limit 1
```

`ci.yml` runs vet, race tests, a `go mod tidy` check and `goreleaser check` on every push to `main`.

Done when: on `main`, level with `origin/main`, the tree is clean, and the latest CI run on `main` is green and ran on the current `HEAD`. If any fails, stop and tell the user.

## 2. Pick the version

```bash
git tag --sort=-version:refname | head -5
git log --oneline "$(git describe --tags --abbrev=0)"..HEAD
```

Read `## [Unreleased]` in `CHANGELOG.md` and choose by semver:

- **Patch** (`1.1.0 → 1.1.1`): only `Fixed` / `Security` entries.
- **Minor** (`1.1.x → 1.2.0`): anything `Added`, `Changed`, `Deprecated` or `Removed`, backwards-compatible.
- **Major** (`1.x.x → 2.0.0`): a break users rely on: a flag or command removed or renamed, a `config.yaml` key changed, a different `gamelist.xml` or media layout, or a game cache format that existing caches can't read.

If the user passed a version, use it. Otherwise propose one with a one-line reason.

Done when: the user has confirmed the version. Wait for a yes before editing anything.

## 3. Cut the CHANGELOG section

In `CHANGELOG.md`:

1. Rename `## [Unreleased]` to `## [X.Y.Z] - YYYY-MM-DD` (today, from `date -u +%Y-%m-%d`; no `v`). Put a fresh, empty `## [Unreleased]` above it.
2. Check the section against the commit log from step 2. Fill gaps as one-line entries written for users in `GLOSSARY.md` vocabulary (lookup, match, not found, non-game…), not as commit subjects, in the groups Added, Changed, Deprecated, Removed, Fixed, Security. Leave out refactors, tests, docs-only changes and fixes to things never released.
3. Update the link references at the bottom: `[Unreleased]` compares `vX.Y.Z...HEAD`, and a new `[X.Y.Z]` line compares the previous tag to `vX.Y.Z`.

Done when this prints the new section, the same command the release workflow runs:

```bash
awk -v v="X.Y.Z" 'index($0, "## [" v "]") == 1 {f=1; next} f && (/^## \[/ || /^\[[^]]*\]: /) {exit} f' CHANGELOG.md
```

## 4. Commit, push, tag

```bash
git add CHANGELOG.md
git commit -m "Release vX.Y.Z: <one-line summary>"
git push origin main
git tag -a vX.Y.Z -m "Release vX.Y.Z"
git push origin vX.Y.Z
```

Commit first, then tag, so the tag holds the CHANGELOG section the workflow reads. `git push` never pushes tags; the tag needs its own push, and that push starts the release.

Done when: both pushes succeed.

## 5. Watch the release

```bash
gh run list --workflow release.yml --limit 1
gh run watch <run id> --exit-status
gh release view vX.Y.Z
gh release view vX.Y.Z --json assets -q '.assets[].name'
```

Expect `checksums.txt` and six archives: `scrapegoat_{Linux,Darwin}_{x86_64,arm64}.tar.gz` and `scrapegoat_Windows_{x86_64,arm64}.zip`.

Then confirm the binary carries its version:

```bash
bin=$(mktemp)
gh release download vX.Y.Z -p 'scrapegoat_Linux_x86_64.tar.gz' -O - | tar -xzO scrapegoat > "$bin" && chmod +x "$bin" && "$bin" version
# scrapegoat version X.Y.Z
```

Done when: the run is green, all seven assets are listed, the Release notes are the CHANGELOG section, and the binary prints `scrapegoat version X.Y.Z`.

## When it goes wrong

Report the failing step and its log (`gh run view <run id> --log-failed`), then:

- **Failed before GoReleaser published** (including the notes step, when the CHANGELOG section is missing or empty): nothing is out. Fix on `main`, then, once the user approves, move the tag: `git tag -fa vX.Y.Z -m "Release vX.Y.Z" && git push -f origin vX.Y.Z`.
- **Failed in or after publishing**: a GitHub Release may already exist. Check `gh release view vX.Y.Z` and tell the user what is out there; the usual way on is a new patch version rather than moving the tag.
- **The binary prints `scrapegoat version dev`**: the `-X main.version` ldflag in `.goreleaser.yml` no longer matches the `version` var in `cmd/scraper/main.go`. `go build` does not error on a missing `-X` symbol.
