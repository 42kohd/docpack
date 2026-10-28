---
type: integration
slug: warehouse-a
title: Warehouse A
summary: Forward records to Warehouse A through its batch ingest endpoint, and check them with three ready-made dashboards.
category: Forwarders
icon: assets/icons/warehouse-a.svg
related: [warehouse-b, delivery-guarantees]
---

## Overview

The Warehouse A forwarder sends each batch as one compressed HTTPS request to the batch ingest endpoint. Warehouse A stores the records in a table that you choose. Use it when you want records queryable within a minute and you already run Warehouse A.

## Prerequisites

- A Warehouse A project with a table for log records.
- An ingest token with write access to that table.
- Outbound access from each agent to the ingest endpoint on port 443.

## Configuration

Point the agent at the forwarder in the configuration file. Keep the token in an environment variable, not in the file.

```yaml title="agent.yaml"
output: forward
forward_to: warehouse-a
batch_size: 500
flush_interval: 5s
warehouse_a:
  endpoint: https://example.com/warehouse-a/v2/batch
  table: app_logs
  token_env: WAREHOUSE_A_TOKEN
```

Check the file before you restart the agent:

::: command Validate the configuration
windows: {binary}.exe check --config "C:\ProgramData\{binary}\agent.yaml"
linux: sudo {binary} check --config /etc/{binary}/agent.yaml
macos: {binary} check --config /usr/local/etc/{binary}/agent.yaml
:::

These flags control batching and retries:

{{ flags group="forwarders" }}

## Target configuration

Create the table in Warehouse A before the first batch arrives. The forwarder does not create tables.

```sql title="Create the table"
CREATE TABLE app_logs (
  timestamp   TIMESTAMP NOT NULL,
  message     TEXT NOT NULL,
  source      STRUCT<path TEXT, host TEXT, offset BIGINT>,
  level       TEXT,
  agent       STRUCT<id TEXT>,
  batch       STRUCT<id TEXT>,
  labels      MAP<TEXT, TEXT>,
  multiline   BOOLEAN
) PARTITION BY DAY(timestamp);
```

## Data reference

The forwarder sends the records dataset. Which fields appear depends on `--verbosity`.

{{ fields dataset="records" }}

## Dashboards

::: cards
- [Ingest overview](doc:wa-ingest-overview) — Records per minute, by host and by file.
- [Delivery health](doc:wa-delivery-health) — Queue depth, failed batches and flush time.
- [Source inventory](doc:wa-source-inventory) — Which files each agent reads.
:::

## Capacity

One agent sends about one request per `--flush-interval` when traffic is light, and one request per full batch when it is heavy.

| Records per second | Batch size | Requests per minute |
|---|---|---|
| 50 | 500 | 12 |
| 500 | 500 | 60 |
| 5000 | 500 | 600 |
| 5000 | 2000 | 150 |

::: tip Larger batches for busy hosts
Warehouse A accepts batches up to 10 MB after compression. Raise `--batch-size` on hosts that send more than 1000 records per second.
:::

## Troubleshooting

The agent log names the batch and the response for every failed request.

401 Unauthorized
:   The token is missing or expired. Check that `WAREHOUSE_A_TOKEN` is set in the environment of the service, not only in your shell.

404 Table not found
:   The table in the configuration does not exist. Create it as shown in [Target configuration](doc:warehouse-a#target-configuration).

413 Payload too large
:   A batch passed the size limit. Lower `--batch-size`.

Batches parked in the state directory
:   The endpoint was unreachable for longer than the retries last. The agent sends parked batches when it reconnects. See [Delivery guarantees](doc:delivery-guarantees#retries).

::: warning Clock skew
Warehouse A rejects records with a timestamp more than 24 hours in the future. Keep host clocks synchronised.
:::

## Related

::: cards
- [Warehouse B](doc:warehouse-b) — The other forwarder target.
- [Ship your first file](doc:ship-first-file) — A five-minute test of this forwarder.
- [Delivery guarantees](doc:delivery-guarantees) — Retries, duplicates and restarts.
:::
