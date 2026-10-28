# Writing documentation with docpack

You write each page as a Markdown file in `docs/`. `docpack check` tells you
exactly what to fix, and when the check passes it builds the zip you send.

## The loop

1. **Start a page.** `docpack new howto rotate-output` writes
   `docs/rotate-output.md` with every section the page needs and a `_fill:`
   line in each. Types: `landing`, `concept`, `howto`, `integration`,
   `format`, `reference`, `dashboard`, `release`.
2. **Write it with your model.** Give your assistant `prompts/SYSTEM.md` as
   its instructions and `prompts/new-document.md` as the task, with the
   skeleton and your notes. To move an existing page across, use
   `prompts/convert-existing.md`. If you use Claude Code or Cursor, the
   `CLAUDE.md` here carries the same rules.
3. **Check.** Run `docpack check`. Every problem is one line:
   `docs/rotate-output.md:14  cmd-missing-variant  Command "Restart" has no macos: line. Add macos: <command>, or macos: none: <reason> if the command does not apply there`
4. **Fix.** Apply the fix at the end of each line yourself, or paste the
   output and the page into `prompts/fix-errors.md`. Run the check again until
   it prints `ok`.
5. **Look, then send.** `docpack serve` shows the site at
   http://127.0.0.1:8080. When it looks right, send the zip that `check`
   wrote to `dist/`.

## What is where

- `skeletons/`: one starting page per type (what `docpack new` writes).
- `examples/`: three finished pages, one integration, one howto and one
  format, that pass the check.
- `prompts/`: instructions for a language model.
- `CLAUDE.md`: the same rules for coding assistants.

Add a new page to `nav` in `product.yaml`, or to the `related` list of a page
that is already there. A page nobody can reach is an error.
