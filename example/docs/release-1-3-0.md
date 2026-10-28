---
type: release
slug: release-1-3-0
title: Lanternfish 1.3.0
nav_title: "1.3.0"
summary: Local output rotation, and a clearer name for the batch size flag.
version: 1.3.0
date: 2026-05-20
---

## What's new

- **Output rotation.** `--rotate-size` closes the current output file at a size you choose, and `--rotate-keep` deletes the oldest closed files.
- **Renamed flag.** `--batch-records` is now `--batch-size`. The old name still works in 1.3.0 and logs a warning.

## Fixes

- CSV output quotes fields that contain the delimiter.

## Upgrade notes

- Replace `--batch-records` with `--batch-size` in service definitions and scripts. The old name will be removed in 2.0.0.

## Known issues

- On macOS, `lanternfish service install` must be run with `sudo`, even when the configuration file is in your home directory.
