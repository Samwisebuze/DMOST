# Character schema — v1alpha

`character.schema.json` is the entry point: the document aggregate root for one
player character. It holds only the top-level assembly — every section is a
`$ref` into a sibling file.

| file | holds |
| --- | --- |
| `character.schema.json` | root document: envelope fields plus one ref per section |
| `common.schema.json` | shared vocabulary: `ruleset`, `abilityName`, `masteryProperty`, `contentOrigin`, `sourceRef` |
| `identity.schema.json` | `identity`, `progression` |
| `abilities.schema.json` | `abilities`, `abilityScore`, `abilityAllocation` |
| `origin.schema.json` | `species`, `speciesTrait`, `background` |
| `classes.schema.json` | `classEntry` (subclass, saves, weapon mastery) |
| `features.schema.json` | `feature`, `feat`, `proficiencies` |
| `vitals.schema.json` | `vitals`, `conditionInstance` |
| `combat.schema.json` | `combat`, `resource`, `restState` |
| `spellcasting.schema.json` | `spellcasting`, `spellcastingSource`, `slotPool`, `spellEntry` |
| `inventory.schema.json` | `inventory`, `container`, `inventoryEntry` |
| `currency.schema.json` | `currency` (coins, valuables, treasury) |
| `reputation.schema.json` | `reputation`, `reputationGroup` |
| `build-log.schema.json` | `buildLogEntry` |

## Conventions

- **All files are siblings, and all cross-file refs are relative** —
  `{ "$ref": "common.schema.json#/$defs/sourceRef" }`. Because every `$id` sits
  under the same base (`https://example.com/schemas/srd-5.2.1/`), the same
  relative string resolves correctly whether a tool follows `$id` URIs or the
  filesystem. Keep the directory flat so that stays true.
- **Definitions live in `$defs` only**; no file defines a schema at its own root.
  A file is a namespace, not a type.
- **`common.schema.json` is the only file others may depend on.** Sections do not
  ref each other — cross-section links are by id string (e.g. a class's
  `spellcasting_ref`), which keeps the section files independently loadable.
- Loading the root pulls in every sibling. A validator that cannot fetch by URI
  needs all files registered against their `$id`s before validating.
