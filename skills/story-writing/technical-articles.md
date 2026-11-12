<!-- Edit this in zeabur-oss/.github (skills/story-writing/technical-articles.md); the weekly skills sync overwrites the copy in other repositories. -->

# Technical articles

A technical article explains how something works and why it is built that way: a design, a
system, a comparison of approaches, an incident. It follows the same modes as `SKILL.md`
(revise or draft) and the same voice, with these rules on top. The author's newest instruction
beats this file. Change only what the request covers; do not rewrite other articles on the way.

## Order of work

1. **Read the context.** Read the whole article and the posts it builds on, and note what the
   reader already knows. To match the author's voice, read two of their published articles of
   the same kind. Done when you can name the article's real problem, its examples and the
   author's design choices.
2. **Check the claims.** Keep four things apart: what the upstream project does, what our code
   does, what the author chose, and what is still a plan. Check citations and measurements by
   the rules below. Done when each citation supports the claim next to it and nothing unchecked
   reads as fact.
3. **Rewrite the reasoning.** Let a concrete operation raise the problem, then follow its
   conditions and consequences to the design. Keep code, edge cases, diagrams and the full
   bibliography. Cut sentences that only announce the topic, sum up a feeling for the reader or
   describe how the article is arranged. Done when a reader can follow the example to the reason
   for the design without a slogan in between.
4. **Check the result.** Read the whole piece against the voice rules below. For a site, build it
   and open the page: code, tables, diagrams and footnotes all readable, and every language
   carries the same claims and numbers. Do not add tests on the article's text. Done when the
   page renders and the report lists the changed files and the checks you ran.

## Let the example carry the article

Start from an operation the reader can picture, change one condition, say what happens next,
then explain what the author did about it. For example: the platform creates a Deployment, then
someone adds a sidecar with kubectl, so the platform's record and the cluster disagree, and that
is why the platform reads the cluster directly.

This is a way of reasoning, not a four-part template for each section. A paragraph may be long,
or stop on a concrete result; a section does not need a closing lesson. A heading names the
actual problem and need not be a question.

The author explains choices in the first person ("if these resources are also synced by Argo CD,
I warn before changing them"). Keep conditional judgements as conditional. Never invent
experience for the author: running a cluster in production, taking over one, living through an
incident.

## Natural, not forced casual

Keep the technical terms and the English names; repeating Deployment or selector is fine. Name
the field, the resource and the behaviour directly instead of swapping in folksy paraphrases.
Mix long sentences, parenthetical detail and short ones; do not chop the article into a stream of
chat messages. Keep the author's own asides; add no decoration.

No single word proves a text is machine-written: judge whether it carries information there.
Fix these on sight:

- **"This is not hypothetical."** Give the incident's date, path and impact with a source.
- **A paragraph that previews the article** ("this post covers how the pieces connect and what
  they can and cannot guarantee"). Go straight to the first operation.
- **Negative parallelism** as a habit ("not X but Y", 「不是 X，而是 Y」). Write the actual
  technical difference.
- **Disclaimer paragraphs.** Put a limit that matters to the design into its situation (an edit
  to a shared ConfigMap reaches every service that mounts it; deleting a PVC may or may not keep
  the data). Drop general risk warnings.
- **Explaining the diagram's mechanics.** The text describes the system the diagram shows. An
  operating hint goes in the caption, and one the reader does not need ("details expand on
  click") goes.

Replace personification with the fields that say it:

> Before: Whoever writes someone else's name in their spec is using it.
>
> After: web's `envFrom` lists `app-config`, so web uses that ConfigMap. A Pod's volume names a
> PVC, so we know which disk it mounts.

Tie a category back to an operation:

> `app-config` is used by both web and worker. If the interface shows it as web's setting, you
> may change it in web without noticing that worker changes too. Marked as shared, it lists every
> service that uses it.

## Other platforms and incidents

- Introduce the product by its own needs, design and goals. Do not frame it as another project's
  successor or as the answer to someone's incident, and do not make it look better by belittling
  another platform.
- Incidents, ours or others', may illustrate an attack path or a permission risk. Keep the date,
  the scope of the impact and a primary source, and keep what the source proves apart from your
  own inference. Do not use an incident to sell the product. Do not drop a strong case the
  author chose to keep.
- A goal (free, fully open source) can be stated as a goal; never as done when it is not.
- A release, the source being open and a feature being built are three separate facts. Check the
  current repository and the author's instructions for each; do not infer features from a
  version number, do not publish a checklist of release progress, and do not turn shipped work
  into future work.

## Citations: check the claim, not only the link

Open every source and find the passage or the code that supports the claim next to it. A title,
a search snippet or a working URL is not a check. If the source cannot be read, find a readable
copy or another primary source. If none can be read, the source is not cited; report it, and if
it matters, note it in the repository's internal docs or an issue.

- **Docs and source answer different questions.** For internal behaviour the docs do not
  promise, cite the implementation at a fixed tag or commit, with the file and the function. Not
  a repository's home page, and never present one version's implementation as an API guarantee.
- **Upstream rules and ours are different claims.** A Kubernetes Service selects Pods; matching a
  workload's Pod template against it is our rule. A citation supports the claim it is attached
  to, at the right owner.
- **Keep the conditions.** A custom label needs to be configured; an optional self-heal is not a
  default; "may be overwritten" does not become "is always reverted". One example does not prove
  the behaviour of every tool or every version.
- **The reader can reach the evidence.** Prefer sources with the full text openly readable. For
  a paper, add a link to a legal full-text copy when one exists. Do not add a paper only to
  decorate a description of how a diagram works.
- **Footnotes keep the detail.** Use Markdown footnotes with full IEEE-style entries, DOI and
  link, never a bare title link. Removing a citation removes both its attribution in the text
  and its footnote. Check the citation targets on the built page.

## Performance figures and future work

Start from the real implementation and its existing benchmarks; never a reduced model written
for the article. A complexity claim names its variables, the data distribution, the output size
and what is held fixed. Count collecting references, building relations, sorting and
deduplicating, classifying and the final projection separately; producing one graph does not
make the whole path `O(V + E)`.

With every measurement, give the hardware, the build mode, the program version or a traceable
source of the run, the input's composition, warm-up and sampling, the unit, and whether the
figure includes the API, parsing, the network and allocating the result. Take numbers from real
output. Keep synthetic data and real-cluster runs apart, and say so when the figures vary a lot.
With equal object counts, also compare the number of relations and how much the selectors
overlap. Time and memory are different measurements: no memory figure without a memory
measurement.

Future work follows the cost that was observed: which step changes next and why (for example, a
name index for Service lookup, so each Ingress backend stops rescanning). Keep expected gains
apart from measured ones, promise no size of improvement for work not yet built, and do not turn
an article edit into an optimisation project.

## Languages and edit boundaries

Every language version follows `zeabur-writing`: each reads as written in that language, and all
carry the same conditions, cases, figures and citations. Sync only what the request touches; do
not rewrite the whole set in the name of keeping them aligned.

Keep the article's diagram components, its publication date, its existing links and everything
the change does not touch. A writing guide is not a record of product state: for anything the
repository can answer, read its code and configuration.
