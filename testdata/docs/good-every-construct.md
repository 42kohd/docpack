---
type: reference
slug: good-every-construct
title: Case
summary: A test case.
related: [guide]
status: prerelease
since: "2.0"
nav_title: Good
aliases: [old-good]
---

## Overview

Paragraph with **bold**, *italic*, ***both***, `code`, `--alpha`, `event.name`, a [link](https://example.com/page), a [doc link](doc:guide) and a [section link](doc:guide#advanced). Placeholders: {product}, {short}, {binary}, {version}, {support_email}.
A second line joins the paragraph, and an escaped \* stays literal.

### Lists

- One
- Two
    - Nested
        - Deeper
- Three

1. First
2. Second

### Steps

::: steps
### Prepare

Text.

```yaml title="config.yaml"
key: value
# ::: inside code is not a container
```

::: warning Careful
Inside a step.

::: command Nested command
windows: {binary}.exe run
linux: {binary} run
macos: none: Not supported.
:::
:::

### Finish

Done.
:::

## Data

Term
:   Its definition.

Second term
:   Another definition.

| Mode | Meaning |
|---|---|
| a | `x \| y` |
| b | Two |

{{ flags group="core" }}

{{ fields dataset="events" }}

{{ platforms }}

{{ datasets }}

{{ releases limit=1 }}

![A picture](assets/pic.svg "Caption here.")

::: tip Callout
- A list in a callout.
:::

::: important Last
Text.
:::

## Related

::: cards
- [Guide](doc:guide) — How it works.
- [Advanced](doc:guide#advanced) — The advanced part.
:::
