---
type: dashboard
slug: wb-agent-fleet
title: "Agent fleet (Warehouse B)"
nav_title: "Agent fleet"
summary: "See every agent that reported in the last hour, with its input files and queue."
target: warehouse-b
dashboard_id: agent-fleet
---

## Purpose

Use this dashboard to confirm a rollout: every host you installed should appear in **Agents reporting** within two minutes of starting.

## Queries

The panels use these queries. Copy one into a new panel to change its time range or grouping.

```sql title="Panel queries"
COUNT DISTINCT agent.id FROM agent_metrics BY 5m
LAST input_files FROM agent_metrics BY agent.id
MAX queue_depth FROM agent_metrics BY agent.id
```

## Related

::: cards
- [Warehouse B](doc:warehouse-b) — Configure the streaming forwarder.
:::
