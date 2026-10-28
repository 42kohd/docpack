---
type: release
slug: release-1-4-0
title: Lanternfish 1.4.0
nav_title: "1.4.0"
summary: Multiline records, a flush time metric, and compressed batches to Warehouse B by default.
version: 1.4.0
date: 2026-09-15
---

## What's new

- **Multiline records.** Set `--multiline-start` to a pattern that marks the first line of a record, and following lines join it. This is a prerelease feature in 1.4.0.
- **Flush time metric.** The agent-metrics dataset reports `flush_ms`, the average time from a batch closing to the target accepting it.
- **Compression to Warehouse B.** Batches to Warehouse B are compressed by default, as they already were for Warehouse A.

## Fixes

- A file renamed and recreated within one poll interval is no longer read twice.

## Upgrade notes

- Agents that set `--compress=false` keep their setting. Agents that never set it start compressing after the upgrade, which lowers bytes sent by about 80 percent.
