# Changelog

Dates are UTC.

## 0.1.1 — 2026-10-28

- `go install github.com/42kohd/docpack/cmd/docpack@<version>` is the primary
  install; `docpack version` reports the module version for such builds.
- Releases sign `checksums.txt` with cosign keyless signing
  (`checksums.txt.sigstore.json`) and attach SLSA build provenance
  attestations to every archive. README: "Verifying what you run".

## 0.1.0 — 2026-10-28

First release.

- The docpack dialect and package format, schema version 1.0 (SPEC.md).
- `docpack init`, `check`, `pack`, `new`, `serve` and `version`.
- JSON Schemas for the manifest, documents, blocks, text runs, product and data files.
- The author kit: skeletons for eight document types, three examples, model prompts.
- An example project for a fictional log-shipping agent.
