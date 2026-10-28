# Decisions

Choices made while building docpack, and why. Newest at the bottom.

## D1. "Advisories" instead of the release section named in the brief

The brief listed a release-note section and a `releases.yaml` key whose name it
also banned from the repository. The banned-word rule is the stricter
constraint, so the section is **Advisories** and the key is `advisories[]`
(`{id, text}`). Same purpose: notices a reader must act on.

## D2. Schema `$id`s and the versioned schema directory

The schemas live in `schemas/1.0/`, one directory per package `schema_version`.
Every `$id` is the raw URL of its file on `main`, for example
`https://raw.githubusercontent.com/42kohd/docpack/main/schemas/1.0/block.schema.json`,
so a consumer can fetch them directly. The checker never fetches them: they
are embedded, and `schemas.Version` and `schemas.BaseURL` are the only
constants in code. A new schema version gets a new directory; published
directories are not changed.

## D3. Go module path

The module is `github.com/42kohd/docpack`, the repository that hosts it. To
move it, run `go mod edit -module <path>` and search-and-replace the import
prefix.

## D4. The package is self-contained

`reference_table` blocks carry their resolved `columns` and `rows`, and the
manifest carries the resolved navigation. A consumer can render any document
without re-implementing filters, generated groups or release ordering.
`data/*.json` is still shipped for consumers that want the raw facts.

## D5. Rule ids live in SPEC.md part C2 and in code

`internal/report/rules.go` is the registry; a test fails when the two disagree
or when a rule has no fixture under `testdata/`.

## D6. One extra package: `internal/project`

The brief's tree had no home for loading `product.yaml`, the data files and
the asset list, and both `rules` and `compile` need them. `internal/project`
holds that loading and the navigation resolver; `compile` runs the pipeline
(load, rules, compile, validate, zip) so the CLI, the server and the tests
share it.

## D7. Skeletons shipped with the checker commit

`docpack new` embeds `kit/skeletons/*.md` (through the tiny `kit` package), so
the skeletons had to exist for the step-3 commit to build. The rest of the kit
landed in its own step.

## D8. Text diagnostics mark warnings in the message

The required line shape is `file:line  rule  message. fix`, which has no
severity column. Warnings keep that shape and prefix the message with
`warning: `; JSON output has a real `severity` field. Diagnostics are sorted
by file, line and rule, and exact duplicates are dropped.

## D9. `pack` is `check` for scripts

Both write the zip. `pack` prints only the zip path on stdout and sends
diagnostics to stderr, so `zip=$(docpack pack)` works in a script.

## D10. Dashboard documents get generated sections

*Panels* (the screenshot plus a `panels` reference table) and *Fields used*
(a `dashboard_fields` table resolved across datasets) are inserted after
*Purpose* from `data/dashboards/<target>.yaml`. These two sources exist only
for generated sections; authors cannot write them.

## D11. Small points the brief left open

- **Flag spans** are code spans starting with `-` and then a letter, so `-1`
  is not a flag. The name ends at the first `=` or space.
- **`levels[]`** on a field lists the output verbosity levels (`minimal`,
  `standard`, `verbose`) that emit it, matching a `--verbosity` style flag.
- **`typical_count`** on a dataset is records per agent per day.
- **Nav items** are a slug, `{doc, children}`, or a nested group; generated
  groups are `{group, from, limit}`. Depth counts the top-level group as 1.
- **Release order** is by version, newest first, wherever releases are listed.
- **Steps** take step headings at `###`–`#####`; a step title that starts with
  a number is an error, because the renderer numbers steps.
- **Cards** require an em dash between the link and the text, as written in
  the construct table.
- **Nested lists** must be indented exactly 4 spaces per level (the table says
  so); CommonMark would accept 2.
- **Content before the first `##`** is an error for every type, including
  `landing`. The `summary` is the lead.
- **Skeleton values.** Frontmatter checks skip values that still contain
  `_fill:`, so an untouched skeleton fails only with `fill-left` (tested).
- **Missing data files** count as empty lists; only `product.yaml` is
  required. A data file that fails to parse or validate silences the
  cross-file checks that depend on it, so one mistake gives one error.

## D12. The package is reproducible

Zip entries are sorted and carry a fixed 1980-01-01 timestamp, documents are
marshalled with stable key order, and the manifest has no build time. The same
input gives the same bytes (tested), so a consumer can use the hash to skip
unchanged imports.

## D13. `serve` re-reads the project on every request

Edits show on reload with no watcher. Check problems are listed at the top of
every page instead of stopping the server, so authors can see their page while
fixing it. Commands render as CSS-only tabs (no JavaScript).

## D14. Toolchain and CI

`go.mod` declares Go 1.22, the oldest version the three dependencies allow.
CI uses GitHub-hosted runners (`ubuntu-latest`) and the official goreleaser
action. `.goreleaser.yaml` builds linux, darwin and windows for amd64 and
arm64; archives include the kit and schemas. `release.yml` publishes a draft
release when a `v*` tag is pushed. No tag has been created.

## D15. `docpack init` and `_fill:` outside documents

`docpack init <short-name>` writes a complete project whose only problems are
`_fill:` markers: in `product.yaml` (name, binary, version, support_email) and
in `docs/index.md` (the landing skeleton, slug `index`, which the generated
nav lists). The nav has two groups, "Get started" and a generated "Releases"
group that stays empty until release documents exist. Data files start as
empty lists. To make that hold, `fill-left` also scans `product.yaml` and
`data/**/*.yaml` (comment lines excepted), schema checks skip values that
still hold `_fill:`, and the literal-name checks wait until the name is real.
`testdata/init/filled/` holds the filled-in files, and a test proves both
states.

## D16. Install with go install; verify releases with cosign and provenance

`go install …/cmd/docpack@<tag>` is the primary install, since it builds from
the tagged source and the Go checksum database pins the module. The release
archives, with the kit and schemas, are the fallback for machines without Go.
`docpack version` falls back to the module version recorded by Go when
goreleaser did not stamp one.

Releases are verifiable two ways, both through official actions and both with
no long-lived keys:

- goreleaser signs `checksums.txt` with cosign keyless signing
  (`sigstore/cosign-installer`). The bundle `checksums.txt.sigstore.json`
  holds a certificate for the identity of `release.yml` at the tag.
- `actions/attest-build-provenance` records SLSA build provenance for every
  archive and for `checksums.txt`, which `gh attestation verify` checks.

Only `checksums.txt` is signed. Each archive is covered through its checksum
and its own attestation. CI runs `goreleaser check`; the signing and
attestation steps themselves only run on a tag push.
