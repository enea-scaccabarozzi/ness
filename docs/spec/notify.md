> [!NOTE]
> Archive: published as written for the public release; only this note was added.

# Notify (NTF)

Packages: `internal/atom/{slack,telegram,github,command}`, cli `post`. Sources: §2 (channel seam),
§5 (`ness post`), §9, D13.

## `ness post`

- **R-NTF-001** `ness post` MUST take `--channel NAME --file MD` and optionally `--thread KEY` and
  `--dry-run`.
  Verify: testscript — missing `--channel` or `--file` → refused.
- **R-NTF-002** `ness post --channel NAME` MUST refuse a name not in `[notify.channels]`.
  Verify: testscript — unknown name → exit 4 listing configured channels.
- **R-NTF-008** Any session (and the operator) MAY run `ness post`.
  Verify: testscript — a child posts successfully to the fake channel.
- **R-NTF-003** `--dry-run` MUST print the exact payload(s) that would be sent, one per message
  part, and MUST make no network call.
  Verify: testscript — fake HTTP server records zero requests; stdout equals what a real post
  sends (compared with the fake's capture of a non-dry run).
- **R-NTF-004** When `note.<channel>.md` exists next to the `--file` note, it MUST be sent
  verbatim for that channel instead of the rendered input.
  Verify: testscript — `note.slack.md` present; the fake captures its bytes unchanged.
- **R-NTF-005** `--thread KEY` MUST post as a reply in the thread mapped to KEY; the first post
  with a new KEY MUST start the thread and record the mapping.
  Verify: contract(fake+real) — second post with the same key carries the first's thread id.
- **R-NTF-006** `ness post` MUST be separate from `ness send` (external posts are irreversible):
  `send` MUST never post to a channel.
  Verify: testscript — `send` to any recipient; fake channels record nothing.
- **R-NTF-007** A multi-part post MUST record each part as posted in the store; a failed post MUST
  exit non-zero naming the parts already sent, and a retry of the same post MUST resume at the
  first unposted part, never reposting a sent part.
  Verify: contract(fake) — fake fails part 2 of 3; stderr says part 1 sent; retry → fake receives
  parts 2 and 3 only.

## Rendering and splitting

- **R-NTF-010** Each channel atom MUST render the one Markdown input into that platform's format.
  Verify: unit — golden test per construct per channel (R-NTF-030).
- **R-NTF-011** When the rendered message exceeds the channel's limit, it MUST be split only on
  paragraph and code-fence boundaries.
  Verify: unit + fuzz — every part ≤ the limit; no part starts or ends inside a code fence or
  paragraph; parts concatenated equal the full render.
- **R-NTF-012** When a single block (paragraph or code fence) exceeds the limit on its own, the
  post MUST fail before sending anything, naming the block.
  Verify: unit — one 5,000-char paragraph to telegram → error; fake records zero posts.
- **R-NTF-013** The Markdown parser/renderers MUST NOT panic on any input.
  Verify: fuzz — arbitrary Markdown into each renderer.

## Channels (§9 table)

- **R-NTF-020** Slack MUST render to `markdown` blocks.
  Verify: unit — payload contains only `markdown` blocks; golden per construct.
- **R-NTF-021** Each Slack message MUST carry at most 12,000 characters of `markdown` block text
  and at most 50 blocks; longer input is split into several messages per R-NTF-011.
  Verify: unit — 12,000 chars → one message; 12,001 → two; 51 blocks → two.
- **R-NTF-022** Slack MUST render Markdown tables as code blocks.
  Verify: unit — golden.
- **R-NTF-023** Slack threads MUST be addressed by the thread ts recorded for the key.
  Verify: contract(fake+real) — reply carries `thread_ts`.
- **R-NTF-024** (CHANGED: spans inside bold and italic) Telegram MUST render in HTML parse mode
  (not MarkdownV2); the spans inside bold and italic are rendered too, so `**[t](url)**` is a bold
  link. A backslash before `\`, backtick, `*`, `_`, `[` or `]` is that character as text, also in
  bold, italic and link text.
  Verify: unit — payload `parse_mode = HTML`; golden per construct; a bold and an italic link, an
  emoji link label, a list and the blank line between blocks rendered exactly; an escaped title
  renders literally inside a bold link.
- **R-NTF-025** Telegram parts MUST be at most 4,096 UTF-16 code units measured after entity
  parsing.
  Verify: unit — text whose HTML source exceeds 4,096 but visible text does not → one part;
  2,049 astral-plane emoji (4,098 units) → two parts.
- **R-NTF-026** Telegram MUST render tables as `<pre>`.
  Verify: unit — golden.
- **R-NTF-027** Telegram threads MUST be a reply or forum topic addressed by the key.
  Verify: contract(fake+real) — reply has `reply_to_message_id` or `message_thread_id`.
- **R-NTF-028** GitHub MUST post GFM as is, at most 65,536 characters per part, tables native;
  the thread key is the issue or PR number.
  Verify: unit — payload body equals input; contract(fake+real) — `--thread 42` comments on #42.
- **R-NTF-029** The `command` channel MUST pass raw Markdown on stdin and the thread key in
  `NESS_THREAD_KEY`, with no length limit.
  Verify: testscript — command `cat > out; env > env`; out equals input; env has
  `NESS_THREAD_KEY`.

## Golden tests

- **R-NTF-030** Each renderer MUST have a golden test per supported construct (at least:
  heading, paragraph, bold, italic, inline code, code fence, link, list, table, the characters
  that need escaping).
  Verify: review — golden files present for each construct × channel.

## Decided details

- **R-NTF-040** The thread key → platform thread id mapping MUST be stored in the store per
  channel.
  Verify: testscript — after a first post, a store dump has the mapping; a second process reuses
  it.
- **R-NTF-041** Slack limits MUST be counted in characters (Unicode code points); GitHub limits
  in characters.
  Verify: unit — multibyte input at the limit fits in one part.
- **R-NTF-042** Channel credentials MUST be resolved from the channel's references (R-CFG-080,
  R-NTF-060); a missing one MUST refuse before any network call.
  Verify: testscript — `token = "env:SLACK_TOKEN"` with the variable unset → exit 4; fake server
  sees no request.

## Secrets (CX-22)

- **R-NTF-060** A channel MUST be built from its keys with every reference resolved (R-CFG-080)
  at the moment it is built, by the worker and by one-shot commands alike, so neither needs the
  value in its environment. A dry run MUST NOT need the referenced values.
  Verify: testscript — `token = "secret:tg"` set with `ness secret set` and an empty environment
  → `ness post` sends with the token; `--dry-run` with the secret unset → exit 0, no request.
- **R-NTF-061** Every value a channel got from a reference MUST be masked in the error text of a
  failed post.
  Verify: testscript — a command channel whose output echoes a referenced value → stderr masks it.
