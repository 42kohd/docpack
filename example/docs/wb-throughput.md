---
type: dashboard
slug: wb-throughput
title: "Throughput (Warehouse B)"
nav_title: "Throughput"
summary: "Follow the records and bytes that reach Warehouse B over time."
target: warehouse-b
dashboard_id: throughput
---

## Purpose

Use this dashboard to size the collection and to spot hosts that suddenly send more. **Records read versus sent** shows a gap when agents queue records they cannot deliver yet.

## Queries

The panels use these queries. Copy one into a new panel to change its time range or grouping.

```sql title="Panel queries"
SUM records_sent FROM agent_metrics BY 1m
SUM bytes_sent FROM agent_metrics BY 1m
SUM records_read, SUM records_sent FROM agent_metrics BY 5m
```

## Related

::: cards
- [Warehouse B](doc:warehouse-b) — Configure the streaming forwarder.
:::
