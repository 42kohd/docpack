# Writing docs in this project (for Claude Code, Cursor and similar tools)

This project's documentation is written in the docpack dialect and checked with
`docpack check`. The full rules are in `kit/prompts/SYSTEM.md` of the docpack
download; read it before editing any file in `docs/`. The short version:

- Start a project with `docpack init <short-name>` and a document with
  `docpack new <type> <slug>`. Never create one by hand.
- Replace every `_fill:` line. Never add, rename, remove or reorder `##`
  sections, and never add frontmatter keys.
- Use only the constructs in the table in SYSTEM.md. No HTML, no emoji, no `#`
  headings, no `•` bullets, no blockquotes, no relative or `.html` links.
- Write `{product}`, `{binary}`, `{version}`, `{short}`, `{support_email}`
  instead of the literal values.
- Commands are `::: command Caption` blocks with `windows:`, `linux:` and
  `macos:` lines; `none: <reason>` when one does not apply.
- Link documents with `doc:slug` or `doc:slug#section-id`.
- Never type flag, field or platform tables; use `{{ flags group="…" }}`,
  `{{ fields dataset="…" }}`, `{{ platforms }}`.
- Facts about flags, fields, platforms, datasets, dashboards and releases live
  in `data/`. Change them there, not in prose.

After every edit, run `docpack check`. Each error line ends with the fix to
apply. Repeat until it prints `ok`. Do not edit `dist/`.
