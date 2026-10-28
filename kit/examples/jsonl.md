---
type: format
slug: jsonl
title: JSON Lines output
summary: The local output format that writes one JSON object per record, one record per line.
related: [rotate-output]
---

## Overview

With `--output=file` and `--output-format=jsonl`, the agent writes each record as one JSON object on its own line. Files are UTF-8 and end every line with a newline.

## Best for

- Hosts that cannot reach a warehouse, where another tool collects the files later.
- Keeping a local copy for a short time while you test a forwarder.
- Loading records into tools that read JSON Lines directly.

## Usage

Select the format with two flags:

::: command Write JSON Lines files
windows: {binary}.exe run --config agent.yaml --output file --output-format jsonl
linux: {binary} run --config agent.yaml --output file --output-format jsonl
macos: {binary} run --config agent.yaml --output file --output-format jsonl
:::

Files go to `--output-dir`. To limit their size, see [Rotate output files](doc:rotate-output).

## Schema

Each line holds the fields of the records dataset. Fields without a value are left out, not written as null.

{{ fields dataset="records" }}

## Examples

A record at the standard level:

```json title="One line, formatted for reading"
{
  "timestamp": "2026-03-14T10:22:31.104Z",
  "message": "GET /health 200 3ms",
  "source": {"path": "/var/log/app/web.log", "host": "web-07"},
  "level": "info",
  "agent": {"id": "a1f4c9"},
  "labels": {"team": "payments"}
}
```

## Notes

- Nested names in the field table, such as `source.path`, are nested objects in the file.
- The order of keys within a line is not fixed.

::: note CSV is also available
`--output-format=csv` writes the same fields as columns. Nested fields become columns named with dots.
:::

## Related

::: cards
- [Rotate output files](doc:rotate-output) — Keep output files to a fixed size.
- [How it works](doc:how-it-works#records) — Where the fields come from.
:::
