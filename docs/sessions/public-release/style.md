> [!NOTE]
> Archive: the record of the session that prepared the public release. Names of private projects were replaced; the content is otherwise as written.

# Writing style for the public docs and the landing page

The operator's requirement (PR-D5): the docs must not read as LLM-generated. Use short, direct
sentences and structured formatting, apply technical-writing best practice, and write for the
reader of each page. This file becomes the contributor style guide (CONTRIBUTING) at release.

## Readers, page by page
| Where | Reader | What they want | Shape |
|---|---|---|---|
| Landing | A developer scanning for 10 seconds | What is it, why should I care, is it real | Headline, one line under it, visuals, proof numbers |
| `why/` | A sceptical senior dev deciding whether to adopt | The argument and the trade-offs | Short claims, each backed by a fact or an example |
| `start/` | A new user at a terminal | Commands that work the first time | Numbered steps, one action each, expected output |
| `concepts/` | An adopter building a mental model | How the parts fit | A definition first, then a diagram or table, then the details |
| `guides/` | A practitioner with a goal | The steps to the goal | A task title, prerequisites, steps, a check that it worked |
| `reference/` | Someone looking one thing up | The exact fact | Tables and lists, no prose arguments |
| `built-with-ness/` | Evaluators, contributors, hiring managers | Evidence | Numbers with their sources, what failed and what was fixed |

## Rules
1. Lead with the point. The first sentence of a page or section says what it is or what to do.
2. One idea per sentence, usually under 25 words. Paragraphs of at most three sentences.
3. Second person, present tense, active voice: "ness opens a worktree", not "a worktree will be
   opened".
4. Use a list for three or more parallel items, a table for comparisons, and numbered steps for
   procedures.
5. Headings: tasks in guides ("Freeze a file"), nouns in concepts and reference ("Probes"). Use
   sentence case.
6. Every command is copy-pasteable, was run by hand, and is logged in `verification.md`. Show the
   expected output after it.
7. Define a term once, where it first appears, and link to that definition afterwards. Use the
   glossary terms and no synonyms: session, root, child, operator, worker, profile, slot, atom,
   seam, feature, gate, rung, size.
8. Use concrete numbers with their sources. Never "many", "significantly" or "dramatically".
9. State limits plainly ("Linux only", "experimental"). Do not hedge everything else.
10. On the landing page, be punchy but factual: every claim must be one the docs prove.

## Banned (typical LLM tells)
- Words: seamless, robust, powerful, leverage, empower, unlock, elevate, delve, streamline,
  supercharge, game-changer, cutting-edge, effortless, simply, just, easy, crucial,
  comprehensive, ecosystem (unless literal).
- Phrases: "Whether you're X or Y", "In today's fast-paced…", "It's important to note", "Let's
  dive in", "not just X, but Y", "In summary / In conclusion", "Here's the thing", "At its core".
- Patterns: em-dash chains, triads used as padding, rhetorical questions, exclamation marks,
  emoji in prose, closing paragraphs that restate the page, every bullet starting with a bolded
  label plus a colon, sentences that announce what the next sentence will say.

## Code blocks
Give every block a language tag, and a title when it shows a file. Keep a command and its output
in separate blocks or label them clearly. Use no `$` prompt in copyable commands.
