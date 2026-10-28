# docpack: write your product docs

You write each page as a Markdown file, `docpack check` tells you exactly what
to fix, and you send one zip. The full dialect is in [SPEC.md](../SPEC.md).

**1. Install.** With Go 1.22 or later:
`go install github.com/42kohd/docpack/cmd/docpack@v0.1.0`.
Without Go, download your OS's archive and `checksums.txt` from
<https://github.com/42kohd/docpack/releases> (`arm64` for Apple silicon and
ARM, `amd64` otherwise), check it (`sha256sum --ignore-missing -c checksums.txt`;
on macOS `shasum -a 256 …`; see "Verifying what you run" in the
[main README](../README.md) for Windows and signatures), then put it on your PATH:

- Windows (PowerShell): `Expand-Archive docpack_*_windows_amd64.zip $HOME\docpack; [Environment]::SetEnvironmentVariable("Path", "$env:Path;$HOME\docpack", "User")`
- macOS: `mkdir -p ~/docpack && tar -xzf docpack_*_darwin_arm64.tar.gz -C ~/docpack && xattr -cr ~/docpack && echo 'export PATH="$HOME/docpack:$PATH"' >> ~/.zshrc`
- Linux: `mkdir -p ~/docpack && tar -xzf docpack_*_linux_amd64.tar.gz -C ~/docpack && echo 'export PATH="$HOME/docpack:$PATH"' >> ~/.bashrc`

Open a new terminal and run `docpack version`. This kit is the `kit/` folder
of the archive, or of the repository if you used `go install`.

**2. Start a project.** In an empty folder run `docpack init <short-name>`
(for example `docpack init acme`), then replace every `_fill:` value in
`product.yaml`.

**3. Add a page.** `docpack new <type> <slug>`, for example
`docpack new howto install-agent`. Types: `landing`, `concept`, `howto`,
`integration`, `format`, `reference`, `dashboard`, `release`. Add the slug to
`nav` in `product.yaml`.

**4. Write it.** Replace every `_fill:` line yourself, or use a model: give it
`prompts/SYSTEM.md` as its instructions, then paste `prompts/new-document.md`,
the skeleton and your notes. In Claude Code or Cursor, copy `CLAUDE.md` into
your project. `examples/` has three finished pages.

**5. Check, fix, repeat.** Run `docpack check`. Each problem is one line that
ends with the fix, for example
`docs/install-agent.md:14  cmd-missing-variant  Command "Start" has no macos: line. Add macos: <command>, or macos: none: <reason> if the command does not apply there`.
Fix it, or paste the output and the page into `prompts/fix-errors.md`. Repeat
until it prints `ok`.

**6. Preview.** `docpack serve`, then open <http://127.0.0.1:8080>.

**7. Send.** Send `dist/<short>-<version>.zip` to your docs maintainer.
