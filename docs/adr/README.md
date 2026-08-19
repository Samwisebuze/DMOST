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

Records 0002–0011 all descend from `docs/psd/0001_character-management-tui.md` and are meant to be read
against it: the PSD carries the product decisions D1–D27, these carry the ones that span packages,
change a published contract, or reverse an earlier record.

| # | title | status |
| --- | --- | --- |
| [0001](0001-character-sheet-tui.md) | Character sheet TUI | Accepted, superseded in part |
| [0002](0002-character-document-store.md) | Align the character stack with PSD-0001, breaking it where needed | Proposed (v0.4) |
| [0003](0003-generated-types-as-representation.md) | Generated schema types are the in-memory representation | Proposed (v0.3) |
| [0004](0004-play-log-schema-change.md) | `play_log` enters `character.schema.json` as Phase 0 pre-work | Proposed (v0.2) |
| [0005](0005-derivation-boundary.md) | Derived values are arithmetic over inputs, never rules tables | Proposed (v0.2) |
| [0006](0006-undo-snapshots-restore.md) | Undo is in-session; history is snapshots that write forward | Proposed (v0.3) |
| [0007](0007-autosave-and-validation-gates.md) | Autosave is unvalidated; validation gates the boundaries | Proposed (v0.2) |
| [0008](0008-cli-shape-and-ambient-resolution.md) | One binary, cobra subtrees, and ambient target resolution | Proposed (v0.1) |
| [0009](0009-testing-strategy.md) | Five test layers, with the round-trip corpus load-bearing | Proposed (v0.2) |
| [0010](0010-freeze-dmostd.md) | Freeze `dmostd` until LAN server work begins | Proposed (v0) |
| [0011](0011-open-schemas-through-v1alpha.md) | Character schemas stay open through `v1alpha` | Proposed (v0) |

## Drafting

A record under `Proposed` carries a version in its front matter and a **Revisions** list under Status,
because the reasoning in a proposal is still being worked on and the series' value depends on showing
that. `v0` is the initial draft; each later revision is one review pass and one commit. Once a record is
`Accepted` the version stops moving — from then on the rule at the top of this file applies and the only
edits are typos and a supersession pointer.
