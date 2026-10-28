---
type: integration
slug: warehouse-b
title: Warehouse B
summary: Forward records and agent metrics to Warehouse B through its streaming endpoint.
category: Forwarders
icon: assets/icons/warehouse-b.svg
related: [warehouse-a]
---

## Overview

The Warehouse B forwarder streams batches to a Warehouse B collection. It sends two datasets: the records themselves and one agent-metrics entry per agent per minute. Use it when you want to watch the agents as well as the logs.

## Configuration

```yaml title="agent.yaml"
output: forward
forward_to: warehouse-b
warehouse_b:
  endpoint: https://example.com/warehouse-b/collections
  records_collection: logs
  metrics_collection: agent_metrics
  key_env: WAREHOUSE_B_KEY
```

::: command Restart the agent after editing
windows: Restart-Service {binary}
linux: sudo systemctl restart {binary}
macos: sudo launchctl kickstart -k system/{binary}
:::

Batches are compressed by default; set `--compress=false` only when a proxy cannot pass compressed requests.

## Data reference

Besides the records dataset, the forwarder sends agent metrics:

{{ fields dataset="agent-metrics" }}

## Queries

Find agents that are falling behind:

```sql title="Agents with a growing queue"
SELECT agent.id, MAX(queue_depth) AS depth
FROM agent_metrics
WHERE timestamp > NOW() - INTERVAL 15 MINUTE
GROUP BY agent.id
HAVING depth > 1000
ORDER BY depth DESC
```

## Dashboards

::: cards
- [Throughput](doc:wb-throughput) — Records and bytes over time.
- [Errors and retries](doc:wb-errors-and-retries) — Failed batches and retries by agent.
- [Agent fleet](doc:wb-agent-fleet) — Every agent that reported in the last hour.
:::

## Troubleshooting

No agent metrics arrive
:   The metrics collection does not exist or the key cannot write to it. Records still arrive, because each dataset is sent separately.

Gaps of exactly one minute
:   The agent restarted. Metrics for the minute of the restart are not sent.

## Related

::: cards
- [Warehouse A](doc:warehouse-a) — The batch forwarder.
- [Command-line reference](doc:cli-reference#forwarder-flags) — Every forwarder flag.
:::
