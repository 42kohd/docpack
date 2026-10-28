---
type: howto
slug: ship-first-file
title: Ship your first file
summary: Send the records of one log file to Warehouse A and find them in the warehouse.
task: Forward one log file to Warehouse A
related: [install, warehouse-a]
---

## Goal

Records from one log file arrive in Warehouse A, and you can query them there.

## Prerequisites

- {product} is installed. See [Install the agent](doc:install).
- A Warehouse A ingest token, stored in the environment variable `WAREHOUSE_A_TOKEN`.

## Steps

::: steps
### Pick a file

Choose a log file that grows while you work, such as a web server access log. Note its full path.

### Run the agent in the foreground

Start the agent with flags instead of a configuration file. `--read-from=start` sends the lines already in the file.

::: command Forward one file to Warehouse A
windows: {binary}.exe run --input-path "C:\logs\web.log" --forward-to warehouse-a --read-from start
linux: {binary} run --input-path /var/log/app/web.log --forward-to warehouse-a --read-from start
macos: {binary} run --input-path /usr/local/var/log/web.log --forward-to warehouse-a --read-from start
:::

The agent prints one line per batch it sends.

### Query the records

In Warehouse A, count the records from your file:

```text title="Warehouse A query"
count(records) where source.path = "/var/log/app/web.log" every 1m
```

::: tip Stop with Ctrl+C
The agent sends its open batch before it exits, so no records are lost when you stop it.
:::
:::

## Result

The query returns one row per minute with the number of records sent. New lines in the file appear within `--flush-interval`, five seconds by default.

## Options

- Add `--input-path` again to follow more files.
- Set `--verbosity=verbose` to include `source.offset` and `batch.id` in each record.

## Related

::: cards
- [Warehouse A](doc:warehouse-a) — Configure the forwarder for production.
- [How it works](doc:how-it-works) — What happens between the file and the warehouse.
:::
