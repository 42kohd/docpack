---
type: dashboard
slug: wb-errors-and-retries
title: "Errors and retries (Warehouse B)"
nav_title: "Errors and retries"
summary: "Find where batches fail and how often agents retry."
target: warehouse-b
dashboard_id: errors-and-retries
---

## Purpose

Use this dashboard when the throughput gap grows. Failed batches concentrated on one agent point to that host's network; retries on every agent point to the collection or the key.

## Queries

The panels use these queries. Copy one into a new panel to change its time range or grouping.

```sql title="Panel queries"
SUM batches_failed FROM agent_metrics BY agent.id
SUM retry_count FROM agent_metrics BY 5m
COUNT FROM records WHERE level = 'error' BY source.path
```

## Related

::: cards
- [Warehouse B](doc:warehouse-b) — Troubleshoot the forwarder.
:::
