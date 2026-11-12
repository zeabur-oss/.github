---
name: no-ai-flavour
description: Keep code comments, copy and visuals free of the generic "AI-made" feel (AI 味) — over-explained comments, design rationale in code, filler phrasing, macOS window dots, fake terminals, glow-on-dot-grid cards. Use when writing or reviewing code comments, UI copy, READMEs, illustrations, covers or landing visuals, and when briefing agents that will.
---

<!-- Edit this in zeabur-oss/.github (skills/no-ai-flavour/SKILL.md); the weekly skills sync overwrites the copy in other repositories. -->

# No AI flavour

## Comments
- Write only what the code cannot say: a constraint that is not visible, a library or browser
  quirk and its cause, a workaround, a spec link. Usually one line.
- No design rationale, measurements ("4.6:1 contrast", "matches X"), history ("previously…"),
  section banners, restated code or TODO essays. A better name beats a comment.
- A decision people need lives in the README or design doc, briefly, not in the code.
- Sweep: list comments in the diff and delete each one that fails the rule; do not reword it.

## Copy
- Concrete over abstract; the reader's task over the system's internals. Cut "seamlessly",
  "robust", "leverage", "powerful", em-dash chains, rule-of-three lists, rhetorical questions,
  "Not just X, but Y".
- In Chinese: no 被 passives by default, no 請 on every line, no 「進行」 padding, no translated-
  English rhythm. Prefer the term the platform itself uses (Apple's system dialogs for passkeys).
- Sources for review: Wikipedia "Signs of AI writing"; Reinhart et al., PNAS 2025.

## Visuals
- No macOS traffic-light dots or fake window chrome, no invented terminal output, no floating
  cards on a dot grid with purple glow, no gradient meshes, no generic icon clusters.
- Use the product's real output, real files and real states, set in the brand's own type, grid
  and colour. Annotate the real thing instead of mocking a UI.

## When briefing agents
Paste the relevant section above into the brief, and review their output against it before
reporting it as done.
