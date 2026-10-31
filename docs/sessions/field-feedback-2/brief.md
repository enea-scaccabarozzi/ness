> [!NOTE]
> Archive: altered for the public release. Names of private projects were replaced; the content is otherwise as written.

# Brief: field-feedback-2

The operator's words, verbatim.

## Operator, 2026-10-20

As you can see from recent session and commits at the moment we are evaluating how ness behaves in real world project to enhance its capabilities based on grounded results. Last session improved it leveraging results from the research lab first iteration and now i am ready to give you a new batch of feedbacks, and once we have reasoned on them and folded them, grounding them from feeling to data, i would love to start integrating ness usage in the production backend project, which is may last personal project that still needs to be migrated to this tool, and its the most ambitious. More on this last part below. Now the feedbacks and my recent feelings:

- Operator notifications are still not good enough and still ahve some problems: in particulr it sent me many false positive on the idle and trouble specturum, and some smaller hiccups in the research lab recent usage. You ahve to undertans its failure model and fix it. This is the msot clear error/friction that i can find it, and it has a clear expected behaviour brokne. Not much to reason about here on ness phyplosohy, it's more a mechanical fix that needs grounding and debugging and may be complex
- Another more mechanical fix/feature addition is allowing ness to handle multiple parallel releases with their overlapping probing windows, since i fear that this possibility is at the moment impossible in current implementation. This doubt emerged (and may be uncofirmed) in latest interation on the current root session in the production backend custom harness, who told me explicitly that multiple announced releases are impossible

Here below the feedbaks are much more about feelings of mine and needs a different apoproach, more linked to ness philoshosy and the balance between flexibility, cerimories and structure:

- I am not sure that the idea of freezed contract and thus the relative feature presence in ness is actually of any benefit, and it's worth it's cerimony and efficiency loss. In order to answer me i want you to analyze all the contract usage over all my eprsonal projects, comapring the ness profile used in ness project itslef and the research lab with the original implementations by the custom harness shipped (and used much more extensivlely) by the production backend project. I want to understand if they are worth it, what are pros and cons, limitation, if agent can bypass them and whatever else it may be useful to me to know. The results of this analysis coukld open a refinement on the contract feature in ness and profile that are using it, and shape the production backend ness integration path
- The other conceptual doubt of mine is linked with execution modes and in particular is plitted in 2 halfs, that i think are related, at leas partially:
    - mainly related with the recent ness iteration i fear that ness's usefulness is archived only from feature and greater complxity execution models. Using ness for less improitatn tasks is only creating cerimony which is not worth its compleicty and overhead.
    - Another pattern of xecution where i could see ness as a big leverage point in benefit for the project using it is having a sort of monitoring execution mode, which is something like the pattern that the production backend is been doing for the last weeks in its root session. It's purpose is something like monitoring a system (with or without a specific focus) leveraging a mechanical floor of probes to wake up the session and handle/raise notficiations for any possible problem. Like having a team memeber actively monitoring a system that is worth this monitoring cost. I do not know if this is another profile, or what else, if this must be handled at a primiteves level from ness, or its proj responsability. However i see this apttern as a really useful feature for production facing projects especially if paired with the release of something big, as done in the production backend recent cutovers

---

This sessions should follow a similar approach as the one done previousley on ness with recent fixes and updates, and inpecting artifacts, records and transcripts from other sessions in other projects is the main way to ground and explore all this feelings and doubts

---

Once the feedbacks are in, any required ness follow up or update is done and  you have gathered enouigh evidences and you have clearly mapped how current harness is helping in the production backend project you must propose me a way to replace with ness the production backend's custom harness, both for later sessions but also updating current active root (which at the moment is only used as monitoring session for the prod cutover) to use the new ness based harness. I think that this occasion will be the first actual evaluation on the field of the ness probing system, which was totally untested up to this moment.

## Operator, 2026-10-20 (answer to checkpoint m-z7eka3rs)

Perfect i am fully aligned and i agree with you on some paoints: Alerts and Parallel releases, howverver i've some doubts on other points:

- contracts: the question is totally different, is not if used by someone, is if when enabled the friction it adds it helps to improve results in code, catching/preventing actual bugs or jsut adding a friction layer that the agent systematically skips at implementation time. You must recover from past transcripts when this logic was added and why in the production backend and the research lab custom harness, and see if thier implementation of this feature was ever worth it. I think you'll have many data points here especially in the production backend. If this analysses revelals that this approaches actually enforces a literature besta practices for BDD and TDD, and this approach is beneficial also in heavy AI assisted projects, the ness contract feature hgains its presence and must be enalbed and ported in both the research lab and the production backend, in some way, or refined frther first
- Small work: yes you catched the crucial part however you ahve not clear the main mechanism on how this tier is used, which is only used when an operator is doing a request, via a virgin session, that must load ness-entry and size itself depending on the asked size. This should not a non reversible choiice, if something that appears small is rather than grounded as big or viceversa, the session tier could be moved if needed. This include also profile like programs, that provides additionals tiers. After this initialization (and realtive cerimony) if the session requires childs, those childs should not require this kind of choice, since a quick task spwan froma  aprante session is alwasy preferred via subagent, rather than quick size child. I am not sure if i am explaining this correctly
- Montor: correct in priciple, i would only prefer avoiding headless runs and audits, and leveraging always the underlyng session, which is lept open or resumed both for daily (or other frequencies) audits and/or the judges. Also i think that the probes features and config expressability already allow for all the mechanism required for this profile, which is just a description on how to use at best this ystetm for this kind of verifications. I do not think it needs primitives updates on ness side on probes, but this is more a high level description on how to use them best. I think that the best showcase of the best approach on this task is the skeleton provided by the last prod probing window inside the production backend project. The montioring profile should start by reasoning and defining the bets way possible setup of mechanical floor, how to set up judges and semantics audits using the ness probe feature and propse it to the user. I think that this is most likely a skill that will be picked up by any session who has to do an in depth monitoring of something over time, after for example a release or simialr big change. I need this skill to be evalauted and created with /skill-creator:skill-creator skill, not as empty prose edits
- The production backend live root: Root is now idle, and thus you can do a similar pattern as done in first field-feedbacks sessions for the research lab, integrate ness in all its depth on a branch (main maybe) and then tell ongoing root to pull this in and guide it on what is required from him to integrate ness and replace its old known custom harness. In particular it's important to replace the probe system with a similar version using ness

## Operator, third message (2026-10-20)

> At the moment i am doing other things, i'lle get back to you on this later, only thing that worries me is that the changes on ness and harness on consuming projects must run only one with all changes batched toghereter once eveything is settled here.
>
> When i'll be back i'll ask you for questions nad what you have done with full context and then i'll resond to u and then authorize the merge into consuming projects

## Operator, fourth message (2026-10-20, answer to checkpoint m-xk5sjm23)

Ok perfect i've read the artifact you gave me, and here are my feedbacks:

- Sizes. Is this your model? Yes it seems correct, however only actual usage will showcase if this is the correct approach and will not simply introduce other problems
- Closing a quick root: as raccomended is fine
- Gate scope: not fully udnerstood the question, repose it here in chat with more context added
- Contracts: i would love to have an extensibility mechanism that consuming project could use to be free to write any way of custom formats and use them. Its much more expressive rather than `keep-lines`. If a project introduces a good format it may be included in builtins in later releases. 
- Monitor skill: seems correct to me
- The production backend cutover timing: Do it now once all ness changes are done
- the batch: perfect, however  for the research lab i want same pattern as the production backend, there is a live root also there

## Operator, fifth message (2026-10-20)

Another fix to build now is allowing poarallel probes execution, instead of sequentials. Do it before release. Also i am fine with the gate scope variable, it's correct to have

## Operator, sixth message (2026-10-20)

Once everything is in main you are authorized to oort neverything on the 3 consuming project as agreed without my expliti go
