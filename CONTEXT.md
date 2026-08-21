# DMOST

The Dungeon Master's Open Source Toolkit: a local-first toolkit for running and
playing tabletop games under a published ruleset. Everything modeled so far
serves one table's characters.

## Language

### People

**User**:
An account in DMOST, identified by an email address.
_Avoid_: account, member, profile

**Handle**:
A User's chosen public name. Optional, and a User may have none.
_Avoid_: username, nickname, alias

**Player**:
A User in the role of playing a Character in a Campaign.
_Avoid_: owner, participant

**Dungeon Master**:
A User in the role of running a Campaign for its Players.
_Avoid_: DM, GM, game master, referee

### The character

**Character**:
The persistent fictional person a Player plays. Identity, creation time, and
revision belong to the Character, not to its Sheet.
_Avoid_: PC, char

**Sheet**:
The single JSON document holding everything recorded about one Character.
_Avoid_: document, character document, doc, blob

**Section**:
A named top-level part of a Sheet — identity, abilities, combat, inventory, and
the rest. Each has its own schema file.
_Avoid_: block, chunk, fragment

**Build log**:
The append-only sequence of build decisions behind a Character's current state,
kept so the state can be explained and a respec reasoned about.
_Avoid_: history, audit log, changelog

**Respec**:
Rebuilding a Character's accumulated choices, replacing earlier ones.
_Avoid_: rebuild, reroll, retrain

**Party**:
The Characters adventuring together in one Campaign.
_Avoid_: table, group, team

**Campaign**:
The ongoing game one Party plays. Not modeled yet: a Sheet carries a
client-chosen campaign id and nothing owns it.
_Avoid_: game, session, adventure

### Content and provenance

**Ruleset**:
The published game system and revision a Sheet is written against — a system
name paired with a revision, e.g. `dnd5e` at `srd@5.2.1`.
_Avoid_: system, edition, rules version

**Snapshot**:
A copy of rules text embedded in a Sheet beside its source ref, so the Sheet
reads without reaching a catalog.
_Avoid_: cached text, inlined rules

**Source ref**:
The provenance block a snapshotted entity carries: where its text came from and
what may be done with it.
_Avoid_: citation, attribution, reference

**Content origin**:
The provenance class of snapshotted content — authored by the User, from an SRD
release, licensed by the operator, or imported with unverified provenance.
_Avoid_: source type, provenance

**Homebrew**:
Content that came from no first-party catalog. Derived from content origin,
never asserted on its own.
_Avoid_: custom content, user content

### Which "version"

Four separate things wear the word. Each has its own name; none of them is
plain "version" in prose.

**Revision**:
The Character's own revision, advanced by a write and used to refuse a lost
update. The only revision a Sheet has — a Sheet carries no counter of its own.
_Avoid_: doc revision, sequence, generation

**Schema version**:
The version of the Sheet contract a given Sheet is written to.
_Avoid_: sheet version, format version

**Wire version**:
The version of the request and response contract clients speak, e.g. `v1alpha`.
_Avoid_: API version, DTO version

**Ruleset revision**:
The revision of the published Ruleset, e.g. `srd@5.2.1`. Always qualified —
never bare "revision", which is the Character's.
_Avoid_: SRD version, rules revision
