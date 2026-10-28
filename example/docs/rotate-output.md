---
type: howto
slug: rotate-output
title: Rotate output files
summary: Keep local output files from filling the disk by closing them at a fixed size and keeping only the newest ones.
task: Rotate the local output files
related: [jsonl]
---

## Goal

The agent writes local output files that never grow past a size you choose, and old files are removed automatically.

## Prerequisites

- The agent writes local files: `--output` is `file`.

## Steps

::: steps
### Choose the limits

Decide the largest file you want and how many closed files to keep. The disk space used is at most the size times the number kept, plus the open file.

### Set the flags

Add the two rotation flags to the service definition or the configuration file.

::: command Rotate at 128 MB and keep 10 files
windows: {binary}.exe run --config agent.yaml --rotate-size 128 --rotate-keep 10
linux: {binary} run --config agent.yaml --rotate-size 128 --rotate-keep 10
macos: {binary} run --config agent.yaml --rotate-size 128 --rotate-keep 10
:::

### Restart the agent

Restart the service so the new limits apply. The open file is closed at the next record that would pass the limit.
:::

## Result

The output directory holds at most 11 files: ten closed files and the one being written. Closed files are named with the time they were closed.

{{ flags group="output" }}

## Related

::: cards
- [JSON Lines output](doc:jsonl) — The format of each output file.
:::
