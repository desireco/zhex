# Zhex versioning policy

Adopted 2026-11-14 in [#82](https://github.com/desireco/zhex/issues/82). The four points below are locked; this page records them and pins the one implementer decision the ticket left open.

1. No version numbers anywhere until the standard library reaches a 0.1 release. Before that: main-only, no tags, no CHANGELOG.
2. From 0.1: 0.x semver — MINOR bump for breaking changes (0.x convention), PATCH bump for additions.
3. Git tags per release; CHANGELOG.md hand-written per release (no automation).
4. No release automation (tag workflows, crates/publishing) before 0.1.

## CHANGELOG.md

CHANGELOG.md first appears with the 0.1 release. No placeholder file is created before then, and nothing links to one.

## Scope

The policy governs releases of the language and of this repository. The unpublished editor extension's local package version (see [editors.md](editors.md)) describes that package, not a language release.

What exactly constitutes the 0.1 milestone is the owner's decision, tracked in [#82](https://github.com/desireco/zhex/issues/82); this page does not define it.
