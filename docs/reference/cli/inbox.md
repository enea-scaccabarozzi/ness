---
title: "ness inbox"
description: "List your open items, one line each; --show ITEM... or --unread prints them in full"
---

## Usage

```text
ness inbox [flags]
```

## Examples

```sh
# what waits for you
ness inbox
# read one in full
ness inbox --show m-abcd2345
# read several, or every unread one, in full
ness inbox --show m-abcd2345 m-efgh6789
ness inbox --unread
```

## Flags

| Flag | Default | Description |
|---|---|---|
| `--boundary NOTE` |  | record NOTE: your state is written down; compact me when idle |
| `--json` |  | print the list as one JSON object |
| `--show ITEM` |  | print the full body of ITEM (more items may follow) and nothing else |
| `--unread` |  | print every open item not shown yet in full, each under a header line |
| `--until TIME` |  | with --boundary: parked until TIME (RFC 3339, YYYY-MM-DD or YYYY-MM-DDTHH:MM, UTC) |

## Inherited flags

| Flag | Default | Description |
|---|---|---|
| `--ui string` | `auto` | renderer: auto, rich or plain (also NESS_UI) |
