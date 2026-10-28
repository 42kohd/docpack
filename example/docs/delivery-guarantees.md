---
type: concept
slug: delivery-guarantees
title: Delivery guarantees
summary: The agent delivers every record at least once, and this page explains what that means for duplicates and restarts.
related: [how-it-works]
---

## At least once

The agent delivers each record **at least once**. A batch counts as delivered only when the target confirms it. If the confirmation is lost, the agent sends the same batch again, so the target can receive a record twice.

Each record carries `batch.id` at the verbose level. A repeated batch keeps its identifier, which lets you remove duplicates in the warehouse.

## Retries

A failed batch is retried up to `--retry-max` times. The wait doubles after each attempt, starting at one second and stopping at 60 seconds. With the default of 8 attempts, the agent tries for a little over two minutes.

| Attempt | Wait before it |
|---|---|
| 1 | none |
| 2 | 1 second |
| 3 | 2 seconds |
| 4 | 4 seconds |
| 5 to 8 | 8 to 60 seconds |

After the last attempt the batch is parked in the state directory. The agent keeps reading new lines and retries parked batches every minute.

## Restarts

On a clean stop the agent sends open batches and writes its offsets. After a crash it starts from the last offsets it wrote, which can resend up to one `--flush-interval` of records.

::: important Order is per file
Records from one file arrive in order. Records from different files can arrive in any order, so sort by `timestamp` in the warehouse.
:::

## Related

::: cards
- [How it works](doc:how-it-works) — The pipeline and the state directory.
- [Warehouse A](doc:warehouse-a#troubleshooting) — Find out why batches fail.
:::
