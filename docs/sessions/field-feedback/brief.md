> [!NOTE]
> Archive: altered for the public release. Names of private projects and local paths were replaced; the content is otherwise as written.

# Brief: field-feedback

The operator's words, verbatim, in order. The proposal they agreed to (2026-10-15 09:34, "Yes, fully agree") is quoted after them.

## Operator, 2026-10-15T08:52:17.152Z

As you can see from current roadmap of v1 version of ness (you may also look at the previous root session which was handling the release/development of this v1) we just finished writing the first rough pass of this tool and now we are using it in different real world projects to collect some feedbacks.

Currently i am tryng to use it in the research lab project `~/src/research-lab` and i started hitting the first frictions:

- the telegram (and other channels) notifications to operator should be richer using better formatting and being more captivating. This is jsut a small thing that is more UI not an actual friction
- A far more improtant thing that you can extract from the last root session running ness in that porj is what i expect from a program profile. In particular the program profile used there is a custom version for research work but the overall idea is the same with some variation:
    - initial brainstorming and high level plan definition is the most criucial part and is almost impossible to be one shotted by the agent, the refinement loop is foudnamental and should be deeply integrated with the profile
    - While the execution of this plan evolves the root should not follow the program blindly, it should instead challenge it deeply and in a socratic way  every time new informations comes in, not vice versa adapting the info to the program. In the case of the research shaped program this requirement is deeper and more important but even in classical program profile for software development this mentaly is crucial
    - The communication at checkpoints is really imporant, and it should be constent, and clear on this 3 crucial points: what was done in that checkpoint, what is the current siotuation and what are the planned next steps
    - Another important question is autonomy, which as opposed as we assumed in first draft of the program profile is difficoult to assume x project and program itslef. So i would love to have it as clear question after the roadmap is planned and agreed on or at any cehckpoint that may change something, since in some long works the user may want to analyze each checkpoint results (like in the research lab), in others it could allow full autonomy if adherent within bands from agreed plan, and in some others it may want to let it go freely until a checpoint specifi, than wait then resume or any other custom setup. It's really a thing that must be settled program execution x program execution, and cannot be fully extracted in the proifile itslef
- Other than that i am pretty sure that if you analyze deeply the transcript of the current root session of the research lab and its childs, you can understand more deeply the expressed feeling above, groundign to my actual messages and steering there, that have been distilled in this core points. Other than that i am sure you can cath more subtle frctions points, like actual failures from ness and hanging handoffs or communciatiosn, or something structurally failing in the research lab, for example wasted time on non relevant hooks or checks duplicated (for example i am pretty sure that all the MDs check that we are doing are jsut slowing evrything down and not worth at all). I am talking on things that slows down the process and piles up creating confusiuons
- also a smaller failure is that sometimes operator messages are not sent and i feel like that root especially is a bit loosing trace of the ness requirements and starts inventing its workflow

---

Your task is to understand deeply this problem by looking at the research lab relevant transcripts with a lot of depth, tryng to find root cause and separating symptmps from them and reporting everything to me, with a proposed solution approach, that span both at ness level (here) and into the research lab local usage of it or setup

## Operator, 2026-10-15T09:27:09.338Z

I fully agree with you and i like very much the suggestions, howver i would love to have some cleaner infos on some parts that seems stranger to me:

- docs/program/doctrine.md: explain who own this file, what goes there and why not inside the program itslef. ALso i suspect that an heavy limitation is that doctrine could change from program execution from program exec ution, even in same project
- Liveness: i feel like that all this patterns must be analyzed further and also the solution itself needs further groundign. Semems a bit naive to me and optimistic
- I also fear that now you are focusing a bit much on the program big size, but ness should work even in self contained tasks that can be done via a quikc root and some childs at most, and the even quiker tiers (where ness may actually not be needed at all)


Also why this session has not adopted ness since it's evident that this work requires it? This should be a root session that works directly here or similar

## Operator, 2026-10-15T09:34:54.200Z

Yes, fully agree

## Agreed proposal (root, 2026-10-15)

Promote as a root here, scope `field-feedback`, tier `full`. First the analysis record (report, the condensed evidence, these clarifications) goes into the session directory. Then the liveness census: every idle interval over about 10 minutes across all 9 research-lab sessions, classified as waiting on a reply, a child, compute, a prompt, or dead. Then brainstorming the designs (checkpoint, autonomy per run, doctrine at two levels, review challenge step, liveness) with the operator before any plan. Program machinery must cost nothing below program size; fixes that help every size come first.
