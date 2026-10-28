---
type: dashboard
slug: wa-ingest-overview
title: "Ingest overview (Warehouse A)"
nav_title: "Ingest overview"
summary: "See how many records reach Warehouse A, and from which hosts and files."
target: warehouse-a
dashboard_id: ingest-overview
---

## Purpose

Open this dashboard first when you add a host or a file. A host that stops sending shows as a drop in **Records by host**, and a quiet file disappears from **Busiest files**.

## Queries

The panels use these queries. Copy one into a new panel to change its time range or grouping.

```text title="Panel queries"
count(records) every 1m
count(records) by source.host top 10
count(records) by source.path top 10
count(records) by level
```

## Related

::: cards
- [Warehouse A](doc:warehouse-a) — Configure the forwarder these panels read from.
:::
