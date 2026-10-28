---
type: concept
slug: how-it-works
title: How it works
summary: The path a record takes from a line in a file to a stored record, and what the agent remembers on the way.
related: [delivery-guarantees]
---

## The pipeline

Every agent runs one pipeline with three stages.

Inputs
:   Each input follows the files that match `--input-path`. The agent notices new lines through file events, or by checking every `--poll-interval` where events are not available.

Batcher
:   New lines become records. The batcher closes a batch when it holds `--batch-size` records or when its oldest record has waited `--flush-interval`.

Output
:   A closed batch goes to one output: a forwarder target, or local files. The output retries a failed batch and parks it in the state directory after `--retry-max` attempts.

![The agent reads inputs, batches records and sends them to one output](assets/diagrams/pipeline.svg "One pipeline per agent.")

## Records

A record is one line, or several lines joined by `--multiline-start`. The agent adds fields that say where the line came from: `source.path`, `source.host` and `timestamp`. The `--verbosity` flag decides how many fields each record carries. The [JSON Lines format](doc:jsonl#schema) lists every field.

## State

The agent keeps two things in `--state-dir`:

- The read offset of every file, so a restart continues where it stopped.
- Parked batches that the output could not deliver.

::: warning Keep the state directory
Deleting the state directory makes the agent forget its offsets. It then starts each file again according to `--read-from`, which can resend or skip records.
:::

## Files that rotate

Log files are often renamed and replaced. The agent tracks each file by its identity on disk, not by its name. A renamed file is read to its end before the agent moves to the new file with the old name.

## Related

::: cards
- [Delivery guarantees](doc:delivery-guarantees) — What the agent promises when the network fails.
- [Install the agent](doc:install) — Run the pipeline on your first host.
:::
