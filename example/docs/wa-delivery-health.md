---
type: dashboard
slug: wa-delivery-health
title: "Delivery health (Warehouse A)"
nav_title: "Delivery health"
summary: "Check that agents deliver on time and find the agents that fall behind."
target: warehouse-a
dashboard_id: delivery-health
---

## Purpose

Use this dashboard when records arrive late. A queue that grows on one agent points to that host; failed batches on every agent point to the endpoint or the token. The panels read the agent metrics, so they need `--verbosity` at standard or above.

## Queries

The panels use these queries. Copy one into a new panel to change its time range or grouping.

```text title="Panel queries"
max(agent-metrics.queue_depth) by agent.id every 1m
sum(agent-metrics.batches_failed) every 5m
avg(agent-metrics.flush_ms) every 5m
```

## Related

::: cards
- [Warehouse A](doc:warehouse-a) — Troubleshoot failed batches.
:::
