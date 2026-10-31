#!/bin/sh
# Trim one round of the 2026-10-31 run (root todo-undo-open, children undo and list-open) into
# site/captures: whole lines only, nothing rewritten; only the CRLF line ends become LF.
# Lines that hold a session URL or a local path (the banner, /remote-control) are never taken.
set -e
C=${CAPTURE_DIR:?set CAPTURE_DIR to the round-hero capture folder}
O=$(dirname "$0")
take() { sed -n "$1" "$C/$2" | tr -d '\r' > "$O/$3"; }
take '3,33p'  2-yes-promotion.ansi             proposal.ansi        # the ask, the proposal, "Start it?" and "❯ yes", one frame (14:07:30)
take '9,21p'  3b-child-undo-tab.ansi           child-undo.ansi      # child undo at 14:08:00: its prompt, then its handoff
take '9,17p'  3c-child-list-open-tab.ansi      child-list-open.ansi # child list-open at 14:08:00
take '1,7p'   4-status-rich-children-live.ansi status.ansi          # ness status at 14:08:00
take '29,43p' 5b-root-asks-to-integrate.ansi   ask.ansi             # the root asks to integrate both (14:10)
take '1,43p'  6-integrate.ansi                 integrate.ansi       # the gate, integrated, the report, the summary (14:11)
