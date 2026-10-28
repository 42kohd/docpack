---
type: dashboard
slug: wa-source-inventory
title: "Source inventory (Warehouse A)"
nav_title: "Source inventory"
summary: "List the files each agent reads and when each file last produced a record."
target: warehouse-a
dashboard_id: source-inventory
---

## Purpose

Use this dashboard to confirm that a new input pattern matches the files you expect, and to find files that have gone quiet.

## Queries

The panels use these queries. Copy one into a new panel to change its time range or grouping.

```text title="Panel queries"
count_distinct(records.source.path) by agent.id
max(records.timestamp) by source.path
```

## Related

::: cards
- [Warehouse A](doc:warehouse-a) — Set up the forwarder and its inputs.
:::
