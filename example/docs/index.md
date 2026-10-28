---
type: landing
slug: index
title: "{product} documentation"
nav_title: Overview
summary: "{product} reads log files on your hosts and delivers each record to a warehouse or a local file."
---

## What {product} does

{product} is a small agent that you run on each host. It follows the log files you name, groups new lines into batches, and sends each batch to one destination. When the destination is unreachable, the agent keeps the batch on disk and tries again.

![The agent reads inputs, batches records and sends them to one output](assets/diagrams/pipeline.svg "Records flow from inputs through the batcher to one output.")

## Start here

::: cards
- [How it works](doc:how-it-works) — The pipeline from a line in a file to a stored record.
- [Install the agent](doc:install) — Put `{binary}` on a host and run it as a service.
- [Ship your first file](doc:ship-first-file) — Send one log file to a warehouse in five minutes.
:::

## Destinations

::: cards
- [Warehouse A](doc:warehouse-a) — Batch ingest over HTTPS, queried with its own language.
- [Warehouse B](doc:warehouse-b) — Streaming ingest with SQL-style queries.
- [JSON Lines output](doc:jsonl) — Write records to local files instead of forwarding them.
:::

## Check your version

::: command Print the installed version
windows: {binary}.exe version
linux: {binary} version
macos: {binary} version
:::

This documentation describes {product} {version}.

## Recent releases

{{ releases limit=3 }}

::: tip Getting help
Write to {support_email} with the output of `{binary} version` and the last 100 lines of the agent log.
:::
