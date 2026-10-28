---
type: reference
slug: cli-reference
title: Command-line reference
nav_title: CLI reference
summary: Every flag of the agent, with its type, default and environment variable, plus the supported platforms and datasets.
---

## Overview

Every flag can also be set with its environment variable or with the matching key in the configuration file. A flag on the command line wins over the environment, and the environment wins over the file.

::: command Show the flags of your version
windows: {binary}.exe run --help
linux: {binary} run --help
macos: {binary} run --help
:::

## Core flags

{{ flags group="core" }}

## Input flags

{{ flags group="input" }}

## Output flags

{{ flags group="output" }}

## Forwarder flags

{{ flags group="forwarders" }}

::: warning Renamed flag
Deprecated flags still work but log a warning on start. Each one names its replacement in the table above.
:::

## Platforms

{{ platforms }}

## Datasets

{{ datasets }}

## Related

::: cards
- [Install the agent](doc:install) — Put the agent on a host.
- [How it works](doc:how-it-works) — What the flags control.
:::
