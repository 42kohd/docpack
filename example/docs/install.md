---
type: howto
slug: install
title: Install the agent
summary: Download the agent, check it runs, and register it as a service that starts with the host.
task: Install the agent and run it as a service
related: [ship-first-file]
---

## Goal

You have `{binary}` installed on one host, running as a service, and starting again after a reboot.

## Prerequisites

- A host on a supported platform. See the [platform list](doc:cli-reference#platforms).
- Administrator rights on the host.
- Outbound HTTPS access if you plan to forward records.

## Steps

::: steps
### Download the package

Download the archive for your platform from the releases page and unpack it.

::: command Unpack the archive
windows: Expand-Archive {binary}-windows-amd64.zip -DestinationPath "C:\Program Files\{binary}"
linux: sudo tar -xzf {binary}-linux-amd64.tar.gz -C /usr/local/bin {binary}
macos: sudo tar -xzf {binary}-darwin-arm64.tar.gz -C /usr/local/bin {binary}
:::

### Check that the binary runs

::: command Print the version
windows: & "C:\Program Files\{binary}\{binary}.exe" version
linux: {binary} version
macos: {binary} version
:::

The command prints the version and the platform. If your shell cannot find the command, add the install directory to your path.

### Create the configuration file

Create a minimal configuration that reads one file and writes local output.

```yaml title="agent.yaml"
inputs:
  - path: /var/log/app/*.log
output: file
output_dir: /var/lib/{binary}/out
state_dir: /var/lib/{binary}/state
```

### Register the service

::: command Install and start the service
windows: & "C:\Program Files\{binary}\{binary}.exe" service install --config "C:\ProgramData\{binary}\agent.yaml"
linux: sudo {binary} service install --config /etc/{binary}/agent.yaml && sudo systemctl start {binary}
macos: sudo {binary} service install --config /usr/local/etc/{binary}/agent.yaml
:::

::: note FreeBSD has no service installer
On FreeBSD, start the agent from your process supervisor with `{binary} run --config <file>`.
:::
:::

## Result

The service is running and writes output files to the directory you configured. Check the service:

::: command Show the service status
windows: Get-Service {binary}
linux: systemctl status {binary}
macos: sudo launchctl print system/{binary}
:::

## Related

::: cards
- [Ship your first file](doc:ship-first-file) — Send records to a warehouse instead of local files.
- [Command-line reference](doc:cli-reference) — Every flag and its environment variable.
:::
