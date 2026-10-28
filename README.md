# docpack

docpack is a documentation format and toolkit: a linter plus a compiler for
product documentation. Authors write pages in a small, closed Markdown dialect
and keep product facts (flags, fields, platforms, releases) in YAML data files.
`docpack check` validates everything and, when it passes, produces one zip
package of schema-validated JSON for any consumer that reads the package
schemas.

## The dialect

A page is one `.md` file with YAML frontmatter and a body built only from
these constructs. Anything else is an error with a line number and a fix.

| Markdown | Block | Rules |
|---|---|---|
| `##`…`#####` | `section` | No `#` in body. Max nesting depth 4. Required section names must match the template exactly. |
| paragraph | `paragraph` | Inline allowed: `**bold**`, `*italic*`, `` `code` ``, `[text](https://…)`, `[text](doc:slug)`, `[text](doc:slug#section-id)`. Nothing else. |
| `-` / `1.` lists | `list` | Nested with 4-space indent. |
| `::: steps` … `:::` containing `### Step title` + content | `steps` | Headed, numbered procedure. |
| `Term` newline `:   definition` | `definition_list` | |
| fenced code with language, optional `title="…"` | `code` | Language is required. |
| `::: command Caption` with lines `windows:`, `linux:`, `macos:` | `command` | All three keys required. A value of `none: <reason>` is allowed and keeps the tab with that note. |
| `::: note\|tip\|warning\|important Title` … `:::` | `callout` | Title required. |
| pipe table | `table` | Max 12 rows. Header row required. All rows same width. |
| `{{ flags group="x" }}`, `{{ fields dataset="x" }}`, `{{ platforms }}`, `{{ datasets }}`, `{{ releases limit=N }}` | `reference_table` | Rendered from `data/`. A hand-typed table whose header matches a reference shape (`Flag`, `Field`, `Platform` first column) is an error. |
| `::: cards` with items `- [Title](doc:slug) — one line` | `cards` | |
| `![alt](assets/path "caption")` | `image` | Alt required; file must exist. |

Each document `type` (landing, concept, howto, integration, format, reference,
dashboard, release) has a fixed list of `##` sections. Placeholders such as
`{product}` and `{binary}` stand in for facts from `product.yaml`, so the
literal product name and version numbers never appear in prose.

## Commands

```text
docpack init <short-name>    create a project in the current directory
docpack check [dir]          check everything; on success write dist/<short>-<version>.zip
docpack pack [dir]           the same for scripts: diagnostics to stderr, zip path to stdout
docpack new <type> <slug>    write a skeleton page to docs/<slug>.md
docpack serve [dir]          render the project at http://127.0.0.1:8080
docpack version              print the version
```

`check` and `pack` take `--format json` (an array of
`{file, line, rule, message, fix, severity}`), `--warnings-as-errors` and
`--out <dir>`. Every diagnostic is one line:

```text
docs/install.md:31  cmd-missing-variant  Command "Start the agent" has no macos: line. Add macos: <command>, or macos: none: <reason> if the command does not apply there
```

## Install

Download the archive for your platform from the
[Releases page](https://github.com/42kohd/docpack/releases) (Linux, macOS and
Windows, each for amd64 and arm64), unpack it into a folder and put that
folder on your PATH. [`kit/README.md`](kit/README.md) has the one-line command
for each OS. With a Go toolchain you can build from source instead:
`go build ./cmd/docpack`.

Each archive unpacks to:

```text
docpack (docpack.exe on Windows)   the binary
kit/                               start here: kit/README.md, then skeletons/,
                                   examples/, prompts/ and CLAUDE.md
schemas/1.0/                       the package JSON Schemas
SPEC.md  README.md  CHANGELOG.md  LICENSE
```

## Start a project

```text
my-docs/
├── product.yaml      name, short, binary, version, support_email, nav
├── docs/             one .md per page
├── data/             flags.yaml, platforms.yaml, datasets.yaml, releases.yaml,
│                     fields/<dataset>.yaml, dashboards/<target>.yaml
└── assets/           images and icons
```

1. In an empty folder, run `docpack init <short-name>`. It writes
   `product.yaml`, empty data files, `docs/index.md` and a `.gitignore`.
2. Fill in `product.yaml`, then run `docpack new howto <slug>` (or another
   type) for each page and add each slug to `nav`.
3. Replace every `_fill:` line. The `kit/` directory has the author's guide,
   three finished examples and prompts for writing pages with a language
   model.
4. Run `docpack check` until it prints `ok`, look at the result with
   `docpack serve`, and hand over the zip from `dist/`.

`example/` is a complete project for a fictional log-shipping agent; run
`docpack serve example/` to see what the output looks like.

## Specification

[SPEC.md](SPEC.md) is normative: the dialect, the data files, the package
layout, the JSON for all twelve block types and the stable rule ids. The JSON
Schemas (2020-12) are in [`schemas/1.0/`](schemas/1.0/); each `$id` is the raw
URL of its file, so the schemas resolve directly. The choices behind them
are recorded in [DECISIONS.md](DECISIONS.md).

## Why the dialect is closed

Open Markdown lets every author invent a layout, and every invention needs its
own styling, its own review and its own handling in every place the docs are
shown. A closed dialect means one theme and one renderer: each construct maps
to exactly one block type, so a consumer renders twelve things and nothing
else, and a page cannot look broken in one place and fine in another. It also
makes the docs friendly to language models. The rules fit in a short prompt, a
skeleton leaves nothing to improvise, and every checker error comes with a fix
that can be pasted straight back into the model.

## License

MIT. See [LICENSE](LICENSE).
