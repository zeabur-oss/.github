---
name: story-writing
description: Write or revise a narrative or technical article (blog post, launch announcement, retrospective, founder letter, an explanation of a design or an incident) so it moves the reader, holds up to a check of its sources and figures, and reads as written by a person. Use when drafting such a piece, when the user asks for 起承轉合, 脈絡, 敘事, or 感動, or when revising one that "reads like AI" (AI 味).
---

<!-- Edit this in zeabur-oss/.github (skills/story-writing/SKILL.md); the weekly skills sync overwrites the copy in other repositories. -->

# Story writing

A narrative article earns its reader with a **spine** (one change the reader should feel), a **turn** (the moment the story bends), and **specifics** only the author could know. Structure it as 起承轉合, then sweep it for AI tells.

## Two modes

- **Revise** (the article exists, someone else wrote it): first read the author's own style guide or two of their published posts, and match that voice rather than a generic blog voice. The author's sentences are the default. Change the fewest sentences that fix a real problem; keep their facts, framing and voice. Propose before applying unless told to apply, and report every changed sentence as before → after. A rewrite of a whole section is a proposal, never an edit.
- **Draft** (nothing exists yet): follow the steps in order.

A technical article (how a design works, why it was chosen, an incident, a comparison or a benchmark) also follows [`technical-articles.md`](technical-articles.md): examples that carry the reasoning, citations opened and checked, measurements with their conditions.

In both modes, every fact, feeling, number and person comes from the author or the source material. Where a passage needs a detail you lack (who was there, what broke, how long it took), ask for it; a placeholder question beats an invented scene.

## Steps

1. **Spine.** Write one sentence: what should the reader feel or do after reading, and why should they believe us? Also note the **stakes**: what it cost, what was at risk, what we cared about. Done when both fit in two sentences the author would sign.
2. **Map 起承轉合.** Label every paragraph with one role:
   - **起**: a concrete opening (a fact, a scene, a problem the reader recognises), never a thesis or a summary of the article.
   - **承**: the work and its context; each paragraph moves the story one step, cause to effect.
   - **轉**: the bend — a cost paid, a plan abandoned, a discovery, a decision that could have gone the other way. One clear 轉 per article; a story with no 轉 is a feature list.
   - **合**: return to the reader. Echo the 起 (the same problem, now answered) and end on one concrete ask or image. No summary of what was already said.
   Done when every paragraph has a role, each role appears, and a paragraph whose role is "repeats an earlier one" is cut or merged.
3. **Order and transitions.** Within 承, order by cause, not by component. Each paragraph's first sentence picks up a thread from the one before (a consequence, a question it raised), so the reader never needs a heading to know why this comes next.
4. **Specifics.** For each paragraph, find the detail only the author knows: a number, a name of a real thing, a user's words, the moment something first worked. Replace an adjective with that detail. Feeling is shown through what people did, not announced ("令人興奮的一步" → what happened when it first worked).
5. **AI-tell sweep.** Read [`ai-tells.md`](ai-tells.md) and resolve every hit in the article. Done when a fresh pass finds none.
6. **Read aloud.** Read the whole piece once as the target reader. Mark any sentence you would skim, any sentence that sounds like a press release, and any stretch of three sentences with the same length and shape; fix those.

## Voice

Plain words, first person plural (我們) for the team, second person (你) for the reader. Mix a short sentence among longer ones. Admit what went wrong in the author's own terms, without self-flagellation and without implying past work was careless. Let some paragraphs end on a fact rather than a reflection.
