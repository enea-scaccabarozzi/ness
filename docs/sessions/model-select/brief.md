> [!NOTE]
> Archive: altered for the public release. Names of private projects were replaced; the content is otherwise as written.

# Brief: model-select

## Operator request (verbatim, 2026-10-21)

I've just merged into main the last set of feedbacks from field into ness. Now i want to do another update and extension to it to accomodate/fix another need of mine. The core problem remaining now is model selection and effort for each child and especially subagents.

From what i see from actual ness execution, especially in finanace project i see that most of the work there is done uniquely on a single model (fable) both in root (where i've selected, probably wrongly, myself and thus is not a ness problem), childs and even most of the subagents. I've worked on this problem before and there should be already some research on where we should use each tier and reasoning effort in each case, and ness should already ship some solution to enforce this, however it simply seems to me that this solkution are skipped in practice.

Ideally i would love if heavier reasoning effort and models were used only in targeted steps (probably only in subagents, and (probably not) targeted special childs). The session should be able to decide himself if that particulr step is worth hevier model and/or heavier reasoning, and then spawn a dedicated subagent to conduct that part, and then prefer cheper defaults for mechanical work. This preference should be confgurable x project, but this should be the sensible defualt. The configurability of this should not done by primarly moving threshold/policies that seprates tiers (which is something that still be possible, maybe in a less direct way), but istead by moving up or down the default tier (which for me it's opus, but for tohers with strictier bidget could be instead something like sonnet)

Another fact that may need additional confirmation is my feeling that reasoning effort is med in 99% of the cases and only in some special cases high, and model selection is the core part of the leverage.

I want you to research best practices here, to optimize tokens and budget, and also to squeeze the most of the performance in every task. Once you have done this research and confirmed, enriched, groudned, or rejected my proposals and feelings i want you to reason about the best way to enforce this choice and patterns found during the execution of long running sessions like the ones that ness creates, since i feel like that this kind of instructions are the first ones that are forgot and lost in context after the first iterations/comptactions.

I also want you to know that i think that this task could benefit a lot from using Jev (from typesafe). This is a new technology that is probably outside your base knowledge, so be sure to ground you on its limits and capabilites to understand what i mean. You should be able to find all the docs u may need suing the istalled skill and/or CC plugin. I imagine this integration shipped as a builtin optional script that can be executed by the agent to decide the correct setup for an input task.
The sauce to obtain the msot of the value from Jev and this models is to engeer in depth the prompt and the questions asked to the model and then combine all the info that we recioeve in code to select the correct setup, even if simply asking the question diretly may be tempting. Fan out pattern is the answer here i think. You'll find the Jev API key inside `/tmp/jev`

Another priciple that i want fully respected and adhered in this task execution is the fact that this model/effort selection module is a fully swappable, agnostic and extensible in project space, primitive of ness.

First of all do the research needed, then come back to me with an approach and we'll refine it togheter before actually committing to it

## Operator follow-ups (verbatim)

- "Perfect i am fully aligned with this, are you sure that the key is still not working, retry. Regarding your question i would start from the stricter refulsal appraoch and decide to loosen it to log only if the data shows too mucb friction"
- "Ok i reminetd one, it should be fine now. Start the session"

## Approved approach (proposed in chat, operator: "fully aligned")

1. One ordered ladder of rungs (model/effort points, claude+codex specs); project sets `base` (and optional `ceiling`) in .ness/ness.toml; selectors return offsets from base, never absolute models.
2. Selector = swappable ness primitive on the D17 router seam: JSON in/out command protocol; built-in `declared` selector (agent names work kind; code maps to offset; offline default) and optional built-in `jev` selector script (fan-out of narrow questions, combined in code); fall back to base on failure/low confidence.
3. Jev fan-out: Scores (reasoning depth, ambiguity, blast radius, novelty, context breadth), Nouls (objective check, mechanical, design-between-alternatives, judge role, long-horizon), Choice (work kind); model offset and effort offset combined separately in code; calibrate on the ~1,300 research-lab Agent calls.
4. Enforcement in code: PreToolUse hook on Agent refuses calls missing model/effort or above the recommended rung (override with logged `escalate: <reason>`) — strict refusal first, loosen to log-only only if data shows friction; `ness open` runs selector on the brief, `--tier` optional, above-base child needs `--reason`; escalate one rung on failed review/verify; SessionStart compact hook re-injects ladder summary; `ness stats` cost by rung and role.

Research files (scratchpad copies): field-models.md, research-tiering.md, sweep-ness-models.md, jev-grounding.md.

Autonomy: act within the brief.
