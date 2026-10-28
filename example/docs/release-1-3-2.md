---
type: release
slug: release-1-3-2
title: Lanternfish 1.3.2
nav_title: "1.3.2"
summary: Parked batches retry after a restart, and the Windows service stops promptly.
version: 1.3.2
date: 2026-07-02
---

## What's new

This patch release changes no defaults. Upgrade every agent running 1.3.0 or 1.3.1.

## Fixes

- Parked batches are retried right after a restart instead of waiting for the next new record.
- The Windows service stops within 10 seconds when the target is unreachable.

## Advisories

- On Windows, versions before 1.3.2 created the state directory with write access for all local users. Lanternfish 1.3.2 resets the access list when it starts. Upgrade, then start the service once.
