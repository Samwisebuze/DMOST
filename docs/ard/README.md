# Architecture Design Records

Numbered, immutable records of decisions that shaped this repository — what was chosen, what it was
chosen over, and what it cost. The `pkg/` doc comments carry the design rationale for code that
exists; these carry the reasoning for decisions that span packages, precede the code, or would
otherwise survive only in a commit message.

## Conventions

- **One decision per file, named `NNNN-kebab-case-title.md`** — four-digit prefix, allocated in
  order, never reused. The number is the record's identity: other documents cite "ARD 0001", so it
  outlives any retitling.
- **The headings are Status, Context, Decision, Consequences**, in that order. Context is the
  situation that forced a choice, stated without the answer in it. Decision is what was chosen,
  in the active voice. Consequences are what follows — the costs as plainly as the benefits.
- **A record is never edited once accepted**, beyond typo fixes. A decision that no longer holds is
  superseded by a new record, and the old one's Status gains a pointer to it. The value of the
  series is that it shows what was believed at the time.

| Status | means |
| --- | --- |
| `Proposed` | written down, not yet agreed |
| `Accepted` | in force; the code either reflects it or is expected to |
| `Superseded by ARD NNNN` | no longer in force, kept for the reasoning |

**Accepted does not mean implemented.** A record describes a decision, not a state of the tree; a
record may sit accepted for a long time before code exists for it, and says so in its own text.

## Index

| # | title | status |
| --- | --- | --- |
| [0001](0001-character-sheet-tui.md) | Character sheet TUI | Accepted |
