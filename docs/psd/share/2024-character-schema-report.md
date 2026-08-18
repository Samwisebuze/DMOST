# D&D 2024 Rules Research & Document-Database Schema for a Character Management Tool

**Target ruleset:** *Dungeons & Dragons* 2024 core rules (Player's Handbook, Sept 2024; Dungeon Master's Guide, Nov 2024; Monster Manual, Feb 2025). D&D Beyond now labels this content "5.5e" and the 2014 content "5e"; Wizards describes it as a revision, not a new edition, and the two are intended to be cross-compatible.

**Licensing baseline:** SRD 5.2 (released 22 April 2025, later 5.2.1) publishes the 2024 core mechanics under CC-BY-4.0. This is the safe surface for shipping rule *content* inside a product — spell text, item text, class tables. Anything outside the SRD (most subclasses, most named magic items, most backgrounds and species options) must be entered by the user or licensed separately. **This constraint shapes the schema more than any other single fact:** the tool cannot assume it owns a complete rules catalog, so character documents must survive with partial, user-supplied, or missing catalog entries.

---

## Part 0 — Design stance

You asked for two things explicitly: no DRY optimization, and no pre-optimization for access patterns. I'm taking both seriously, and the report argues for a schema that is *deliberately* redundant. Three things justify that, independent of performance:

1. **A character sheet is a record of rulings, not a query over a rulebook.** The Longsword on a sheet is not "a pointer to Longsword"; it is "the longsword this DM let this player have, renamed *Dawnbreaker*, currently at 3 charges, with a house rule attached." Late-binding to a catalog row makes the sheet change when the catalog changes. That is a correctness bug, not a normalization win.
2. **The 2024 rules are choice-heavy and mixed-source.** A single table routinely mixes 2014 subclasses, 2024 classes, third-party species, and homebrew. There is no authoritative catalog to normalize against.
3. **Rules text is versioned and errata'd.** SRD version numbers increment (5.2 → 5.2.1 → …). A sheet built in 2025 should still render as it did in 2025.

So the governing pattern is **snapshot + provenance**: every embedded object carries a full copy of what it needs to render and resolve, *plus* a `source_ref` describing where it came from. Duplication is the point. Where I store both a raw input and a computed total, I store both — I do not require the reader to recompute, and I do not require the writer to trust the reader's math.

I use one aggregate root per character. Catalog documents (spell catalog, item catalog, species catalog) exist as separate collections, but they are **seed data**, not runtime dependencies.

---

## Part 1 — Character

### 1.1 What the 2024 rules changed

**Ability scores no longer come from species.** In 2024 the ability score adjustment moved to *background*. Each background lists three ability scores; the player either raises one by 2 and another by 1, or raises all three by 1. Older backgrounds are converted by distributing the same 3 points; if you use an older species with an ASI, you ignore it. This is the single most schema-relevant change in character creation, because the allocation is now a *player decision with two legal shapes*, not a fixed species constant.

**Backgrounds also grant an Origin feat.** Each of the 16 PHB backgrounds bundles: three ability scores, an Origin feat, two skill proficiencies, one tool proficiency, and an equipment package (which can be traded for 50 gp). A custom background framework exists with the same five fixed slots, subject to DM approval. Older backgrounds that don't include a feat get a player-chosen Origin feat.

**Feats are a first-class, four-category system:**

| Category | When acquired | Notes |
|---|---|---|
| Origin | Level 1, from background | 10 in the PHB; can also be taken later |
| General | At ASI levels (4, 8, 12, 16; Fighter also 6 and 14; Rogue also 10) | Most grant a +1 to one ability score alongside a feature; many have a level-4 prerequisite; four are repeatable |
| Fighting Style | Requires the Fighting Style feature (Fighter L1; Paladin/Ranger L2) | Now standalone feats rather than a nested class list |
| Epic Boon | Level 19 only | Can push one ability score above 20, to a maximum of 30 |

Note the cap consequence: the ability-score ceiling is **not a constant**. It is 20 by default and 30 for scores raised by an Epic Boon.

**Species carry no ability scores at all,** and instead give a trait package: creature type, size, speed, senses, and traits. Ten species in the PHB. Several have a mandatory sub-choice that gates traits: Elf picks one of three Lineages, Tiefling one of three Fiendish Legacies, Gnome one of two Lineages, Goliath one of six Giant Ancestries, Dragonborn one of ten Draconic Ancestries. Elf and Tiefling sub-choices grant a cantrip plus level-gated spells at character levels 3 and 5, which are always prepared, castable once per long rest without a slot, and whose spellcasting ability is *chosen by the player at selection time*. Half-Elf and Half-Orc are gone as species; mixed heritage is a descriptive option. Dwarves, gnomes, and halflings all moved to 30 ft speed; Wood Elf and Goliath sit at 35 ft.

**Subclass timing is uniform:** every class chooses its subclass at level 3.

**Exhaustion is now a simple stacking counter:** levels 1–6, each level applying −2 to d20 tests and −5 ft of speed, with death at level 6. It is explicitly the one condition whose effects worsen on reapplication; all other conditions are binary, and multiple instances each track their own duration but do not stack in effect.

**Inspiration became Heroic Inspiration:** expend to reroll any die immediately after rolling, taking the new result; gaining it while you already have it means it is lost unless you hand it to a player character who lacks it. So it is a boolean with a transfer operation, not a counter.

**Other glossary-level facts worth encoding:** proficiency bonus scales +2 through +6 by total character level; carrying capacity is derived from Strength and size; a character may choose to fail a saving throw without rolling.

### 1.2 Schema argument

The character block is where the redundancy pays off hardest. **Store every ability score as a structured object with its full derivation, not a number.** A 2024 character's Strength is the sum of a base value (from standard array, point buy, or rolls), a background allocation, zero or more feat/boon increases, and possibly a magic item override — and the legal maximum depends on whether an Epic Boon touched it. Collapsing that to `"str": 18` throws away the audit trail that lets the tool explain a number to a confused player, and it makes level-up recalculation a guess.

**Store the build as an append-only decision log *and* as a materialized current state.** This is the most emphatically non-DRY choice in the document. The `build_log` records each choice as it was made (species chosen, background ASI split, subclass at level 3, feat at level 4, weapon masteries swapped after a long rest). The rest of the document is the resolved sheet. They will occasionally disagree after a rules errata; that disagreement is *information*, and the log is what makes respec, undo, and "why do I have this?" possible.

**Model species sub-choices as a first-class embedded object,** not a string. `"species": "Elf", "lineage": "Wood Elf"` is not enough, because the lineage grants spells at levels 3 and 5 with a player-chosen spellcasting ability and a free-cast-per-long-rest allowance. Those need to land in the spellcasting section as fully-formed prepared-spell entries with `always_prepared: true` and their own recharge tracking.

**Conditions are a list of instances, not a set of flags** — each with its own source and duration, plus a `level` field used only by Exhaustion.

```json
{
  "_id": "char_7f3a91",
  "doc_type": "character",
  "schema_version": "1.0.0",
  "ruleset": { "system": "dnd5e", "revision": "2024", "srd_baseline": "5.2.1" },
  "campaign_id": "camp_44b1",
  "owner_user_id": "usr_0091",
  "created_at": "2026-08-10T14:02:00Z",
  "updated_at": "2026-08-10T14:02:00Z",
  "doc_revision": 41,

  "identity": {
    "character_name": "Vesk Ambermarch",
    "player_name": "Dana",
    "pronouns": "she/her",
    "alignment": "Chaotic Good",
    "age": "31",
    "height": "5'4\"",
    "weight": "132 lb",
    "faith": "Selûne",
    "appearance": "Freeform text.",
    "backstory": "Freeform text.",
    "roleplay_notes": [
      { "label": "Personality", "text": "Freeform. 2024 backgrounds no longer supply trait/ideal/bond/flaw tables." }
    ],
    "portrait_url": null
  },

  "progression": {
    "total_character_level": 7,
    "experience_points": 23000,
    "advancement_mode": "milestone",
    "proficiency_bonus": 3,
    "epic_boon_taken": false
  },

  "abilities": {
    "strength": {
      "base_score": 10,
      "base_source": "standard_array",
      "background_increase": 0,
      "feat_increases": [],
      "other_increases": [],
      "override_score": null,
      "override_source": null,
      "total_score": 10,
      "modifier": 0,
      "maximum_allowed": 20,
      "save_proficient": false,
      "save_bonus_total": 0,
      "save_misc_bonus": 0
    },
    "dexterity": {
      "base_score": 15, "base_source": "standard_array",
      "background_increase": 1,
      "feat_increases": [{ "feat_instance_id": "featinst_02", "feat_name": "Piercer", "amount": 1 }],
      "other_increases": [],
      "override_score": null, "override_source": null,
      "total_score": 17, "modifier": 3,
      "maximum_allowed": 20,
      "save_proficient": true, "save_bonus_total": 6, "save_misc_bonus": 0
    },
    "constitution": { "base_score": 14, "base_source": "standard_array", "background_increase": 2, "feat_increases": [], "other_increases": [], "override_score": null, "override_source": null, "total_score": 16, "modifier": 3, "maximum_allowed": 20, "save_proficient": false, "save_bonus_total": 3, "save_misc_bonus": 0 },
    "intelligence": { "base_score": 12, "base_source": "standard_array", "background_increase": 0, "feat_increases": [], "other_increases": [], "override_score": null, "override_source": null, "total_score": 12, "modifier": 1, "maximum_allowed": 20, "save_proficient": false, "save_bonus_total": 1, "save_misc_bonus": 0 },
    "wisdom": { "base_score": 13, "base_source": "standard_array", "background_increase": 0, "feat_increases": [], "other_increases": [], "override_score": null, "override_source": null, "total_score": 13, "modifier": 1, "maximum_allowed": 20, "save_proficient": false, "save_bonus_total": 1, "save_misc_bonus": 0 },
    "charisma": { "base_score": 8, "base_source": "standard_array", "background_increase": 0, "feat_increases": [], "other_increases": [], "override_score": null, "override_source": null, "total_score": 8, "modifier": -1, "maximum_allowed": 20, "save_proficient": false, "save_bonus_total": -1, "save_misc_bonus": 0 }
  },

  "ability_allocation": {
    "generation_method": "standard_array",
    "standard_array_assignment": { "15": "dexterity", "14": "constitution", "13": "wisdom", "12": "intelligence", "10": "strength", "8": "charisma" },
    "point_buy_spend": null,
    "rolled_values": null,
    "background_allocation": {
      "pattern": "2_and_1",
      "allocations": [
        { "ability": "constitution", "amount": 2 },
        { "ability": "dexterity", "amount": 1 }
      ],
      "eligible_abilities": ["dexterity", "constitution", "wisdom"]
    }
  },

  "species": {
    "instance_id": "spec_01",
    "name": "Wood Elf",
    "parent_species": "Elf",
    "sub_choice": { "kind": "lineage", "label": "Lineage", "value": "Wood Elf" },
    "creature_type": "Humanoid",
    "size": "Medium",
    "size_options": ["Medium"],
    "base_walking_speed": 35,
    "traits": [
      { "trait_id": "trt_darkvision", "name": "Darkvision", "text_snapshot": "See in dim light within 60 feet as bright light, and in darkness as dim light.", "range_ft": 60 },
      { "trait_id": "trt_fey_ancestry", "name": "Fey Ancestry", "text_snapshot": "Advantage on saves to avoid or end the Charmed condition." },
      { "trait_id": "trt_trance", "name": "Trance", "text_snapshot": "Finish a Long Rest in 4 hours of trance." },
      { "trait_id": "trt_keen_senses", "name": "Keen Senses", "text_snapshot": "Proficiency in Insight, Perception, or Survival (chosen)." , "player_choice": "Perception" }
    ],
    "granted_spell_refs": ["cast_lineage_cantrip", "cast_longstrider", "cast_pass_without_trace"],
    "source_ref": { "catalog_id": "cat_species_wood_elf", "book": "PHB 2024", "page": 187, "is_homebrew": false, "srd_licensed": true }
  },

  "background": {
    "instance_id": "bg_01",
    "name": "Wayfarer",
    "ability_scores_offered": ["dexterity", "wisdom", "charisma"],
    "origin_feat": { "feat_instance_id": "featinst_01", "name": "Lucky" },
    "skill_proficiencies_granted": ["Insight", "Stealth"],
    "tool_proficiency_granted": "Thieves' Tools",
    "starting_equipment_choice": "package",
    "starting_equipment_package": ["Thieves' Tools", "Gaming Set (Dice)", "Bedroll", "2 Pouches", "Traveler's Clothes", "16 GP"],
    "starting_gold_alternative_gp": 50,
    "is_custom": false,
    "custom_definition": null,
    "source_ref": { "catalog_id": "cat_bg_wayfarer", "book": "PHB 2024", "page": 183, "is_homebrew": false, "srd_licensed": false }
  },

  "classes": [
    {
      "instance_id": "cls_01",
      "class_name": "Ranger",
      "class_level": 7,
      "is_starting_class": true,
      "hit_die": "d10",
      "subclass": { "name": "Hunter", "chosen_at_level": 3, "source_ref": { "catalog_id": "cat_sub_hunter", "book": "PHB 2024", "is_homebrew": false } },
      "saving_throw_proficiencies": ["strength", "dexterity"],
      "primary_abilities": ["dexterity", "wisdom"],
      "fighting_style_feat": { "feat_instance_id": "featinst_03", "name": "Archery" },
      "weapon_mastery": {
        "max_known": 2,
        "swap_cadence": "long_rest",
        "known": [
          { "weapon_name": "Longbow", "mastery_property": "Slow" },
          { "weapon_name": "Shortsword", "mastery_property": "Vex" }
        ],
        "last_swapped_at": "2026-08-09T08:00:00Z"
      },
      "spellcasting_ref": "spc_ranger"
    }
  ],

  "features": [
    {
      "instance_id": "feat_ranger_favored_enemy",
      "name": "Favored Enemy",
      "origin": { "kind": "class", "ref": "cls_01", "level_gained": 1 },
      "text_snapshot": "Hunter's Mark is always prepared; a number of free castings per Long Rest.",
      "activation": { "type": "none" },
      "linked_resource_id": "res_favored_enemy",
      "source_ref": { "catalog_id": "cat_feat_favored_enemy", "book": "PHB 2024", "is_homebrew": false }
    }
  ],

  "feats": [
    {
      "instance_id": "featinst_01",
      "name": "Lucky",
      "category": "origin",
      "repeatable": false,
      "times_taken": 1,
      "acquired_at_level": 1,
      "acquired_via": "background",
      "ability_increase": null,
      "prerequisite_snapshot": null,
      "text_snapshot": "Spend Luck Points to gain Advantage or impose Disadvantage.",
      "linked_resource_id": "res_luck_points",
      "source_ref": { "catalog_id": "cat_feat_lucky", "book": "PHB 2024", "is_homebrew": false }
    },
    {
      "instance_id": "featinst_02",
      "name": "Piercer",
      "category": "general",
      "repeatable": false,
      "times_taken": 1,
      "acquired_at_level": 4,
      "acquired_via": "asi_slot",
      "ability_increase": { "ability": "dexterity", "amount": 1 },
      "prerequisite_snapshot": "Level 4+",
      "text_snapshot": "Reroll one damage die on a piercing hit; extra die on a crit.",
      "linked_resource_id": null,
      "source_ref": { "catalog_id": "cat_feat_piercer", "book": "PHB 2024", "is_homebrew": false }
    }
  ],

  "proficiencies": {
    "armor": ["Light", "Medium", "Shields"],
    "weapons": ["Simple", "Martial"],
    "tools": [{ "name": "Thieves' Tools", "source": "background", "expertise": false }],
    "languages": ["Common", "Elvish", "Sylvan"],
    "skills": [
      { "skill": "Perception", "ability": "wisdom", "proficient": true, "expertise": false, "misc_bonus": 0, "total_bonus": 4, "sources": ["species", "class"] },
      { "skill": "Stealth", "ability": "dexterity", "proficient": true, "expertise": false, "misc_bonus": 0, "total_bonus": 6, "sources": ["background"] },
      { "skill": "Survival", "ability": "wisdom", "proficient": true, "expertise": false, "misc_bonus": 0, "total_bonus": 4, "sources": ["class"] }
    ]
  },

  "vitals": {
    "hit_points": { "maximum": 58, "current": 41, "temporary": 0, "maximum_modifier": 0, "hp_roll_mode": "average" },
    "hit_dice_pools": [{ "die": "d10", "class_ref": "cls_01", "total": 7, "remaining": 5 }],
    "death_saves": { "successes": 0, "failures": 0, "stabilized": false },
    "heroic_inspiration": true,
    "conditions": [
      { "instance_id": "cond_01", "name": "Exhaustion", "level": 2, "source": "Forced march", "duration": { "kind": "until_long_rest" }, "applied_at": "2026-08-09T21:00:00Z", "effect_snapshot": "-4 to d20 tests; speed reduced by 10 ft." }
    ],
    "defenses": { "resistances": [], "immunities": [], "vulnerabilities": [], "condition_immunities": [] }
  },

  "combat": {
    "armor_class": {
      "total": 16,
      "calculation": "medium_armor",
      "breakdown": [
        { "label": "Half Plate Armor", "value": 15 },
        { "label": "Dexterity (capped at +2)", "value": 1 }
      ],
      "override": null
    },
    "initiative": { "ability": "dexterity", "bonus": 3, "misc_bonus": 0, "advantage": false },
    "speeds": { "walk": 25, "fly": 0, "swim": 0, "climb": 0, "burrow": 0, "notes": "Base 35, reduced 10 by Exhaustion 2" },
    "size": "Medium",
    "creature_type": "Humanoid",
    "senses": { "darkvision": 60, "blindsight": 0, "tremorsense": 0, "truesight": 0, "passive_perception": 14, "passive_investigation": 11, "passive_insight": 11 },
    "attunement": { "slots_maximum": 3, "slots_used": 2 },
    "carrying": { "capacity_lb": 150, "push_drag_lift_lb": 300, "encumbrance_variant_enabled": false, "current_weight_lb": 74.5 }
  },

  "resources": [
    { "resource_id": "res_luck_points", "name": "Luck Points", "maximum": 3, "current": 1, "recharge": "long_rest", "source_feature": "featinst_01" },
    { "resource_id": "res_favored_enemy", "name": "Hunter's Mark (free casts)", "maximum": 3, "current": 2, "recharge": "long_rest", "source_feature": "feat_ranger_favored_enemy" }
  ],

  "rest_state": {
    "last_short_rest_at": "2026-08-10T11:15:00Z",
    "last_long_rest_at": "2026-08-09T08:00:00Z",
    "last_dawn_at": "2026-08-10T06:04:00Z",
    "pending_long_rest_choices": ["swap_prepared_spell", "swap_weapon_mastery"]
  },

  "build_log": [
    { "seq": 1, "at": "2026-04-01T18:00:00Z", "level": 1, "decision": "species_selected", "payload": { "species": "Elf", "lineage": "Wood Elf" } },
    { "seq": 2, "at": "2026-04-01T18:02:00Z", "level": 1, "decision": "background_selected", "payload": { "background": "Wayfarer", "ability_pattern": "2_and_1", "allocations": [{ "ability": "constitution", "amount": 2 }, { "ability": "dexterity", "amount": 1 }] } },
    { "seq": 3, "at": "2026-05-11T19:30:00Z", "level": 3, "decision": "subclass_selected", "payload": { "class_ref": "cls_01", "subclass": "Hunter" } },
    { "seq": 4, "at": "2026-06-02T19:10:00Z", "level": 4, "decision": "asi_slot_spent", "payload": { "choice": "feat", "feat": "Piercer" } }
  ]
}
```

---

## Part 2 — Items & Consumables

### 2.1 What the 2024 rules require

**Weapon properties** are Ammunition, Finesse, Heavy, Light, Loading, Range, Reach, Thrown, Two-Handed, Versatile. Two changed meaningfully:

- **Heavy** no longer keys off creature size. You have Disadvantage on attack rolls with a Heavy weapon unless you have Strength 13+ (melee) or Dexterity 13+ (ranged). So the property carries a *numeric prerequisite that must be evaluated against the character*, not a static tag.
- **Light** now itself carries the two-weapon fighting rule: attacking with a Light weapon during the Attack action lets you make one extra Bonus Action attack with a *different* Light weapon, with no positive ability modifier on that extra damage.

**Weapon mastery** is the big structural addition. Every weapon in the PHB has exactly one mastery property from a set of eight: Cleave, Graze, Nick, Push, Sap, Slow, Topple, Vex. A character can only use it if they have mastered *that weapon type* (via a class feature or the Weapon Master feat) and are actually wielding it. Masteries are swappable after a long rest. Cleave and Nick are once-per-turn; the others have no usage cap; Topple is the one that forces a save (Constitution vs. Prone).

So the mastery property is a **property of the item**, while "which weapons I have mastered" is a **property of the character** — and the tool must join them at render time. Storing mastery only on the character breaks when a DM hands out an unusual weapon; storing it only on the item breaks the swap-after-long-rest rule.

**Magic items** use six rarity tiers (common, uncommon, rare, very rare, legendary, artifact), plus "varies" for items like Enspelled Armor whose rarity tracks the level of the bound spell. Attunement caps at three items, is established over a short rest, and can be ended voluntarily over another short rest — *unless the item is cursed*, in which case attunement can't be ended voluntarily until the curse is broken. Curses are not revealed by Identify. Attunement prerequisites may require a class, a species, or a spellcasting ability.

**Charges** are a per-item mutable counter with an item-specific recharge expression — commonly "regains 1d6 charges daily at dawn." Enspelled items carry 6 charges. A Wand of Magic Missiles can be used unattuned by anyone and spends 1–3 charges to scale the spell level. So charge state must live on the *instance*, and the recharge rule must live on the instance too (snapshotted), because dawn-based recharge doesn't fire on a rest event.

**Consumables:** drinking a potion or administering it to a creature within 5 feet is a **Bonus Action** in 2024 (a widely-used house rule made official). Applying an oil may take longer per its description. Potions of Healing come in a rarity ladder with escalating dice. Potion mixing has a risk table. The PHB adds nonmagical crafting rules plus crafting for Potion of Healing and Spell Scrolls — meaning items can have a *provenance of "crafted"* with time and gold cost attached.

### 2.2 Schema argument

**Split the item model into a catalog document and an instance document, and let the instance carry a full copy of everything it needs.** The catalog entry is the platonic Longsword. The instance is *this* longsword. The instance embeds the rendered stat block — damage dice, properties, mastery, weight, rarity, attunement requirement, description text — so that deleting or editing the catalog row never mutates a live sheet.

**Use a discriminated union on `item_type` with fully-written-out per-type field sets** rather than a lean generic item with an `attributes` bag. Under a no-DRY mandate this is the right call: a weapon document should literally have `damage_dice`, `damage_type`, `properties`, `mastery_property`, `range_normal_ft`, `range_long_ft`, `versatile_damage_dice`, `ammunition_type`. A consumable should literally have `on_use_action`, `effect_text`, `uses_remaining`. Sharing a bag between them makes validation impossible and forces every reader to know the union's shape anyway.

**Model consumables as items with a `quantity` and an optional `uses` sub-object, not as a separate collection.** A stack of 7 potions and a wand with 4 charges are the same kind of problem — a countable resource attached to a physical object that lives somewhere in the inventory tree. Keeping them in one collection means the "spend a use" operation is uniform. What differs is only *what the count means*: `quantity` decrements to zero and the object disappears; `charges.current` decrements and the object stays.

**Represent Heavy's prerequisite as structured data**, not a string, so the tool can actually warn "you have Disadvantage with this." Same for attunement prerequisites.

```json
{
  "_id": "itm_9a04",
  "doc_type": "item_instance",
  "schema_version": "1.0.0",
  "ruleset": { "system": "dnd5e", "revision": "2024" },
  "owner_character_id": "char_7f3a91",

  "item_type": "weapon",
  "display_name": "Dawnbreaker",
  "base_item_name": "Longbow",
  "quantity": 1,
  "weight_lb_each": 2,
  "weight_lb_total": 2,
  "value_gp_each": 50,
  "is_magical": true,
  "rarity": "rare",
  "rarity_is_variable": false,

  "weapon": {
    "category": "martial",
    "range_type": "ranged",
    "damage_dice": "1d8",
    "damage_type": "Piercing",
    "versatile_damage_dice": null,
    "properties": ["Ammunition", "Heavy", "Two-Handed"],
    "property_details": {
      "heavy": { "melee_requires_strength": null, "ranged_requires_dexterity": 13 },
      "ammunition": { "ammo_item_type": "Arrow", "range_normal_ft": 150, "range_long_ft": 600 },
      "thrown": null,
      "reach_ft": null,
      "loading": false
    },
    "mastery_property": "Slow",
    "mastery_text_snapshot": "On a damaging hit, reduce the target's Speed by 10 feet until the start of your next turn. Does not stack with itself.",
    "attack_bonus_modifier": 1,
    "damage_bonus_modifier": 1,
    "ability_override": null
  },

  "armor": null,
  "consumable": null,
  "container": null,

  "attunement": {
    "required": true,
    "prerequisite": { "kind": "none", "value": null },
    "is_attuned": true,
    "attuned_at": "2026-07-04T09:00:00Z",
    "attuned_by_character_id": "char_7f3a91"
  },

  "charges": {
    "has_charges": true,
    "maximum": 3,
    "current": 2,
    "recharge_expression": "1d3",
    "recharge_trigger": "dawn",
    "last_recharged_at": "2026-08-10T06:04:00Z",
    "destroy_on_empty": false,
    "destroy_on_empty_condition": null
  },

  "curse": { "is_cursed": false, "curse_text_snapshot": null, "curse_revealed_to_player": false },

  "sentience": { "is_sentient": false, "intelligence": null, "wisdom": null, "charisma": null, "alignment": null, "communication": null, "purpose": null },

  "activation": { "action_type": "none", "usage_limit": null, "save_dc": null, "save_ability": null },

  "description_snapshot": "A yewwood longbow chased with silver. Its arrows trail motes of dawnlight.",
  "mechanical_text_snapshot": "You have a +1 bonus to attack and damage rolls made with this magic weapon. Expend 1 charge to make the arrow deal an extra 2d6 Radiant damage.",
  "notes_player": "DM ruled the radiant rider works on Graze misses too. Session 14.",
  "house_rules": [{ "at": "2026-07-04", "ruled_by": "DM", "text": "Radiant rider applies on Graze." }],

  "provenance": {
    "acquired_via": "loot",
    "acquired_at": "2026-07-04T09:00:00Z",
    "acquired_from": "Barrow of the Pale Wardens",
    "crafting": null,
    "identified": true,
    "identified_at": "2026-07-04T20:00:00Z"
  },

  "source_ref": {
    "catalog_id": null,
    "book": "Homebrew",
    "page": null,
    "is_homebrew": true,
    "srd_licensed": false,
    "authored_by_user_id": "usr_dm_0031"
  }
}
```

A consumable instance, same collection, different discriminant:

```json
{
  "_id": "itm_c118",
  "doc_type": "item_instance",
  "item_type": "consumable",
  "display_name": "Potion of Greater Healing",
  "base_item_name": "Potion of Greater Healing",
  "quantity": 3,
  "weight_lb_each": 0.5,
  "weight_lb_total": 1.5,
  "value_gp_each": 100,
  "is_magical": true,
  "rarity": "uncommon",

  "consumable": {
    "consumable_kind": "potion",
    "on_use_action": "bonus_action",
    "can_administer_to_other": true,
    "administer_range_ft": 5,
    "effect_kind": "healing",
    "healing_expression": "4d4 + 4",
    "damage_expression": null,
    "applies_condition": null,
    "duration": { "kind": "instantaneous" },
    "consumed_on_use": true,
    "mixing_risk_applies": true
  },

  "weapon": null, "armor": null, "container": null,
  "attunement": { "required": false, "prerequisite": null, "is_attuned": false },
  "charges": { "has_charges": false },
  "curse": { "is_cursed": false },

  "description_snapshot": "Red liquid that glimmers when agitated.",
  "provenance": {
    "acquired_via": "crafted",
    "crafting": { "tool": "Herbalism Kit", "days_spent": 3, "gp_spent": 50, "crafted_by": "self" },
    "acquired_at": "2026-07-20T00:00:00Z",
    "identified": true
  },
  "source_ref": { "catalog_id": "cat_item_potion_greater_healing", "book": "PHB 2024", "is_homebrew": false, "srd_licensed": true }
}
```

---

## Part 3 — Inventory

### 3.1 What the rules require

Inventory in 2024 is governed by a handful of interacting rules:

- **Carrying capacity** is derived from Strength score and size; the same table gives the push/drag/lift figure. Encumbrance remains a variant, so the tool must support "weight is tracked but not enforced."
- **Containers nest**, and some of them break the physics: a Bag of Holding's contents do not count toward the carrier's load in the normal way. So weight rolls up through the tree *unless a container declares otherwise*.
- **Equipped state is not the same as carried state.** The 2024 rules are generous about drawing and stowing — you can equip or unequip a weapon before or after any attack in the Attack action, and there's still one free object interaction per turn. But the mastery rules require you to be *wielding* the weapon, not merely carrying it, and the Light property requires two different Light weapons actually in hand. Hands are a real, scarce, rules-relevant resource.
- **Attunement** is a character-level cap of 3 that is satisfied by items in the inventory. The count must be derivable but should also be stored.
- **Coins have weight**: 50 coins of any type weigh 1 pound. So currency contributes to encumbrance and must be reachable from the weight calculation.
- **Ammunition** is consumed and partially recovered, so it is a stack with its own semantics.

### 3.2 Schema argument

**Store the inventory as a tree of containers with explicit parent references on each item, and store the resolved location path on the item as well.** Yes, that duplicates. The parent pointer is the source of truth for moves; the denormalized path is what makes a rendered sheet self-explanatory and what survives a container being deleted mid-session.

**Model equipment slots explicitly and separately from containment.** An item is *somewhere* (backpack, chest at the inn, on the ground) and it may *also* be equipped (main hand, off hand, worn armor, attuned). Conflating the two forces awkward hacks like a fake "equipped" container. Hands in particular should be modeled as a two-element array, because the 2024 Light/Nick rules and free-object-interaction rules are hand-count-sensitive.

**Keep a separate `inventory_summary` block with precomputed totals** — total weight, attunement slots used, encumbrance tier. This is redundant with the item list by construction. It is also the thing a DM asks about mid-session, and recomputing it correctly requires knowing every container's weight exemption rules. Store it, timestamp it, and recompute on write.

**Support out-of-body storage.** Items stashed at a bastion, in a vault, on a mount, or in the party's shared cart are inventory the player still wants to see. A `storage_location` enum with `carried | stowed_elsewhere | party_shared | lost | consumed` prevents these becoming ghost items.

```json
{
  "inventory": {
    "summary": {
      "total_weight_lb": 74.5,
      "carried_weight_lb": 74.5,
      "coin_weight_lb": 3.6,
      "carrying_capacity_lb": 150,
      "push_drag_lift_lb": 300,
      "encumbrance_variant_enabled": false,
      "encumbrance_tier": "unencumbered",
      "attunement_slots_maximum": 3,
      "attunement_slots_used": 2,
      "computed_at": "2026-08-10T14:02:00Z"
    },

    "containers": [
      {
        "container_instance_id": "cnt_body",
        "name": "Worn / Carried",
        "container_kind": "person",
        "parent_container_id": null,
        "capacity_lb": null,
        "capacity_cubic_ft": null,
        "contents_are_weightless_to_carrier": false,
        "is_extradimensional": false
      },
      {
        "container_instance_id": "cnt_bp01",
        "name": "Backpack",
        "container_kind": "backpack",
        "parent_container_id": "cnt_body",
        "capacity_lb": 30,
        "capacity_cubic_ft": 1,
        "own_weight_lb": 5,
        "contents_are_weightless_to_carrier": false,
        "is_extradimensional": false
      },
      {
        "container_instance_id": "cnt_boh01",
        "name": "Bag of Holding",
        "container_kind": "magic_bag",
        "parent_container_id": "cnt_body",
        "capacity_lb": 500,
        "capacity_cubic_ft": 64,
        "own_weight_lb": 15,
        "contents_are_weightless_to_carrier": true,
        "is_extradimensional": true,
        "overload_behavior_snapshot": "Exceeding capacity or piercing the bag destroys it and scatters contents in the Astral Plane."
      },
      {
        "container_instance_id": "cnt_vault",
        "name": "Strongbox at the Rusted Anchor",
        "container_kind": "offsite_storage",
        "parent_container_id": null,
        "capacity_lb": null,
        "contents_are_weightless_to_carrier": true,
        "is_extradimensional": false
      }
    ],

    "entries": [
      {
        "entry_id": "inv_001",
        "item_instance_id": "itm_9a04",
        "item_name_cache": "Dawnbreaker (Longbow +1)",
        "quantity": 1,
        "weight_lb_total": 2,
        "storage_location": "carried",
        "parent_container_id": "cnt_body",
        "location_path_cache": ["Worn / Carried"],
        "equipped": {
          "is_equipped": true,
          "slot": "hands",
          "hands_used": 2,
          "is_attuned": true,
          "is_wielded_for_mastery": true
        },
        "sort_order": 10,
        "is_favorite": true,
        "added_at": "2026-07-04T09:00:00Z"
      },
      {
        "entry_id": "inv_002",
        "item_instance_id": "itm_c118",
        "item_name_cache": "Potion of Greater Healing",
        "quantity": 3,
        "weight_lb_total": 1.5,
        "storage_location": "carried",
        "parent_container_id": "cnt_bp01",
        "location_path_cache": ["Worn / Carried", "Backpack"],
        "equipped": { "is_equipped": false, "slot": null, "hands_used": 0, "is_attuned": false },
        "sort_order": 40,
        "is_favorite": true,
        "added_at": "2026-07-20T00:00:00Z"
      },
      {
        "entry_id": "inv_003",
        "item_instance_id": "itm_ammo_arrow",
        "item_name_cache": "Arrows",
        "quantity": 47,
        "quantity_expended_this_session": 13,
        "quantity_recoverable": 6,
        "weight_lb_total": 2.35,
        "storage_location": "carried",
        "parent_container_id": "cnt_body",
        "location_path_cache": ["Worn / Carried"],
        "equipped": { "is_equipped": true, "slot": "quiver", "hands_used": 0, "is_attuned": false },
        "sort_order": 15
      }
    ],

    "equipment_slots": {
      "hands": [{ "slot_index": 0, "entry_id": "inv_001" }, { "slot_index": 1, "entry_id": "inv_001" }],
      "armor": { "entry_id": "inv_010" },
      "shield": { "entry_id": null },
      "head": { "entry_id": null },
      "neck": { "entry_id": "inv_011" },
      "cloak": { "entry_id": null },
      "hands_wear": { "entry_id": null },
      "rings": [{ "slot_index": 0, "entry_id": "inv_012" }, { "slot_index": 1, "entry_id": null }],
      "feet": { "entry_id": null },
      "belt": { "entry_id": null },
      "custom_slots": []
    },

    "transaction_log": [
      { "seq": 88, "at": "2026-08-10T13:10:00Z", "kind": "consumed", "entry_id": "inv_002", "delta": -1, "note": "Used on Brannic after the ogre." },
      { "seq": 87, "at": "2026-08-10T12:44:00Z", "kind": "expended", "entry_id": "inv_003", "delta": -13, "note": "Combat: bridge ambush." }
    ]
  }
}
```

---

## Part 4 — Spells

### 4.1 What the 2024 rules changed

**Every spellcasting class now uses prepared spells.** The known/prepared split is gone. Each class states how many spells it can have prepared (a fixed number from the class table, no longer ability-modifier + level) and how often it can swap. The cadences differ per class and are a genuine schema requirement:

- Wizard prepares from the spellbook and can change the whole prepared list on a long rest; the Memorize Spell feature also allows a swap on a short rest.
- Ranger swaps one prepared spell per long rest.
- Wizard can also replace a known cantrip on a long rest — a wizard-only capability.

**Warlocks still use Pact Magic**, a separate slot pool that recharges on a short rest. That, plus multiclassing, means a character can hold **more than one distinct slot pool simultaneously**. A single `spell_slots[1..9]` array is the classic modeling mistake here.

**Spells cast from non-class sources are common and structurally different.** Species lineages/legacies grant spells at levels 3 and 5 that are always prepared, castable once per long rest without expending a slot, castable normally with slots, and use a spellcasting ability the player picked when choosing the lineage. Feats (Magic Initiate), subclass features, and magic items all do similar things. These are not "prepared spells" in the class sense and must not consume prepared-count budget.

**Rituals** were formalized: a ritual-tagged spell can be cast at +10 minutes without expending a slot, and normally requires the spell to be prepared — with class features like the Wizard's Ritual Adept explicitly waiving that. The ritual tag moved into the casting time line.

**Other structural facts:** casting times of 1 minute or more require the Magic action each turn plus Concentration; casting up-cast means the spell simply takes on the slot's level; there are about 391 spells; emanation is a new area-of-effect shape; and healing spells were rebalanced (Cure Wounds to 2d8 + modifier, Healing Word 2d4 + modifier).

### 4.2 Schema argument

**Model spellcasting as an array of per-source blocks, not as one object on the character.** A Wizard 5 / Warlock 3 has two prepared lists, two preparation rules, two spellcasting abilities, two save DCs, and two slot pools with different recharge triggers. A single-object model forces every consumer into special cases.

**Model slot pools as a list of named pools, each with its own recharge trigger and its own level→count map.** This handles the multiclass spellcaster table, Pact Magic, and one-off pools (like an item that grants slots) with the same shape.

**Give every spell on the sheet a `preparation` object describing *why* it's there.** The five practical origins — prepared from class list, always-prepared from a class feature, granted by species/feat/item, in-spellbook-but-unprepared, and innate free-cast — behave differently at cast time and at long-rest time. Storing an origin enum plus the free-cast allowance on the entry is what makes "you have Longstrider once per long rest without a slot, *and* you can up-cast it with a level 2 slot" expressible.

**Snapshot spell text on the character document.** This is the most disputable non-DRY choice, since 391 spells duplicated across thousands of characters is real storage. I'd still do it, for the same reason as items: DMs alter spells, tables ban spells, errata change spells, and third-party spells have no catalog row. The compromise I'd accept if storage bites: snapshot the *mechanically load-bearing* fields (level, school, casting time, range, components, duration, concentration, ritual, damage/healing expressions, save) and keep only a reference for the long descriptive text.

```json
{
  "spellcasting": {
    "sources": [
      {
        "source_id": "spc_ranger",
        "source_kind": "class",
        "source_name": "Ranger",
        "class_ref": "cls_01",
        "spellcasting_ability": "wisdom",
        "spell_save_dc": 14,
        "spell_attack_bonus": 6,
        "caster_progression": "half",
        "spell_list_name": "Ranger",
        "preparation": {
          "uses_prepared_spells": true,
          "prepared_maximum": 6,
          "prepared_current_count": 6,
          "prepares_from": "class_list",
          "swap_rule": { "cadence": "long_rest", "spells_per_swap": 1, "unlimited": false },
          "can_swap_cantrips": false,
          "cantrips_known": 0
        },
        "ritual_casting": { "enabled": false, "requires_prepared": true, "notes": null }
      },
      {
        "source_id": "spc_lineage",
        "source_kind": "species_lineage",
        "source_name": "Wood Elf Lineage",
        "class_ref": null,
        "spellcasting_ability": "wisdom",
        "spellcasting_ability_was_player_chosen": true,
        "spell_save_dc": 14,
        "spell_attack_bonus": 6,
        "preparation": { "uses_prepared_spells": false, "prepared_maximum": 0, "prepares_from": "granted", "swap_rule": null }
      }
    ],

    "slot_pools": [
      {
        "pool_id": "pool_standard",
        "pool_name": "Spell Slots",
        "recharge_trigger": "long_rest",
        "contributing_sources": ["spc_ranger"],
        "slots": {
          "1": { "maximum": 4, "expended": 1 },
          "2": { "maximum": 3, "expended": 0 },
          "3": { "maximum": 2, "expended": 0 },
          "4": { "maximum": 0, "expended": 0 },
          "5": { "maximum": 0, "expended": 0 },
          "6": { "maximum": 0, "expended": 0 },
          "7": { "maximum": 0, "expended": 0 },
          "8": { "maximum": 0, "expended": 0 },
          "9": { "maximum": 0, "expended": 0 }
        }
      }
    ],

    "pact_magic": null,

    "spellbook": { "has_spellbook": false, "container_item_instance_id": null, "spells": [] },

    "spells": [
      {
        "entry_id": "cast_hunters_mark",
        "spell_name": "Hunter's Mark",
        "source_id": "spc_ranger",
        "preparation": {
          "origin": "always_prepared_feature",
          "origin_detail": "Favored Enemy",
          "is_prepared": true,
          "counts_against_prepared_maximum": false,
          "free_casts": { "maximum": 3, "remaining": 2, "recharge": "long_rest", "cast_at_level": 1 }
        },
        "level": 1,
        "school": "Divination",
        "casting_time": { "action_type": "bonus_action", "value": 1, "unit": "action", "is_ritual": false },
        "range": { "kind": "distance", "distance_ft": 90 },
        "area_of_effect": null,
        "components": { "verbal": true, "somatic": false, "material": false, "material_text": null, "material_cost_gp": 0, "material_consumed": false },
        "duration": { "kind": "timed", "value": 1, "unit": "hour", "concentration": true },
        "attack_or_save": { "kind": "none", "save_ability": null },
        "damage": [{ "expression": "1d6", "type": "Force", "trigger": "on_weapon_hit" }],
        "healing": null,
        "upcast": { "scales": true, "notes_snapshot": "Longer duration at levels 3 and 5." },
        "description_snapshot": "Mark a creature; deal extra damage when you hit it with an attack.",
        "source_ref": { "catalog_id": "cat_spell_hunters_mark", "book": "PHB 2024", "is_homebrew": false, "srd_licensed": true }
      },
      {
        "entry_id": "cast_pass_without_trace",
        "spell_name": "Pass without Trace",
        "source_id": "spc_lineage",
        "preparation": {
          "origin": "granted_species",
          "origin_detail": "Wood Elf Lineage, level 5",
          "is_prepared": true,
          "counts_against_prepared_maximum": false,
          "free_casts": { "maximum": 1, "remaining": 1, "recharge": "long_rest", "cast_at_level": 2 },
          "may_also_cast_with_slots": true
        },
        "level": 2,
        "school": "Abjuration",
        "casting_time": { "action_type": "action", "value": 1, "unit": "action", "is_ritual": false },
        "range": { "kind": "self" },
        "area_of_effect": { "shape": "emanation", "size_ft": 30 },
        "components": { "verbal": true, "somatic": true, "material": true, "material_text": "ashes from a burned leaf of mistletoe", "material_cost_gp": 0, "material_consumed": false },
        "duration": { "kind": "timed", "value": 1, "unit": "hour", "concentration": true },
        "attack_or_save": { "kind": "none" },
        "damage": [], "healing": null,
        "upcast": { "scales": false },
        "description_snapshot": "You and nearby allies gain a bonus to Stealth checks and can't be tracked.",
        "source_ref": { "catalog_id": "cat_spell_pass_without_trace", "book": "PHB 2024", "is_homebrew": false, "srd_licensed": true }
      }
    ],

    "concentration": {
      "is_concentrating": true,
      "spell_entry_id": "cast_hunters_mark",
      "started_at": "2026-08-10T12:44:00Z",
      "ends_at_estimate": "2026-08-10T13:44:00Z",
      "target_note": "Ogre chieftain"
    },

    "pending_swaps": [
      { "source_id": "spc_ranger", "available_at": "next_long_rest", "swaps_allowed": 1 }
    ]
  }
}
```

---

## Part 5 — Currency

### 5.1 What the rules require

Five denominations — copper, silver, electrum, gold, platinum — with the standard 10:1 ladder except electrum's half-gold oddity. **Fifty coins of any type weigh one pound**, which is the rule that forces currency into the encumbrance calculation.

Coins are only part of the treasure picture. The 2024 DMG treats trade bars, gemstones, art objects, and trade goods as distinct treasure categories with their own value tables. Backgrounds allow swapping the equipment package for 50 gp, so starting wealth has two shapes. And downtime systems, crafting, and Bastion upkeep all create scheduled expenditures.

### 5.2 Schema argument

**Store denomination counts as the source of truth. Do not normalize to a single unit.** This is the argument I'd defend hardest in this section. Converting everything to copper on write is tempting and wrong for three reasons: (1) players deliberately hold particular coins, and merchants care; (2) electrum is a house-rule minefield — some tables ban it, some treat it as 5 sp, and a normalized store silently picks a side; (3) conversion at a table is a *negotiated action*, not an arithmetic identity, and a tool that auto-converts will produce balances the player never agreed to.

Store `total_value_gp` and `total_value_cp` as **denormalized convenience fields** computed on write, clearly labeled as derived. That's redundant by design and it's the right redundancy: it lets the UI show a headline number without any consumer re-implementing the conversion table.

**Separate coins from valuables.** A 250 gp ruby is not 250 gp. It is an object with weight, a sale price that depends on a Persuasion check and a market, and possibly a plot attached. Give valuables their own array with an `estimated_value_gp`, a `sold` flag, and a link to an inventory entry if they occupy space.

**Keep a ledger.** An append-only transaction log with running balance snapshots is duplicated state relative to the balances. It is also the only thing that answers "where did 400 gp go" three sessions later, and it makes party-fund splits auditable.

**Model shared party funds as a reference, not a copy.** This is the one place I'd break the self-contained-document rule, because a shared purse copied into five character documents will drift within one session. Reference a `party_treasury` document by id and cache only a read-only snapshot with a timestamp.

```json
{
  "currency": {
    "coins": { "cp": 46, "sp": 12, "ep": 0, "gp": 118, "pp": 4 },
    "coin_count_total": 180,
    "coin_weight_lb": 3.6,

    "derived": {
      "total_value_gp": 130.8,
      "total_value_cp": 13080,
      "conversion_table_used": { "cp_per_sp": 10, "cp_per_ep": 50, "cp_per_gp": 100, "cp_per_pp": 1000 },
      "electrum_enabled_at_table": true,
      "computed_at": "2026-08-10T14:02:00Z"
    },

    "valuables": [
      {
        "valuable_id": "val_01",
        "name": "Star ruby",
        "category": "gemstone",
        "quantity": 1,
        "estimated_value_gp": 1000,
        "appraised": true,
        "appraised_by": "Guild assayer, Session 12",
        "weight_lb_total": 0,
        "inventory_entry_id": "inv_022",
        "is_sold": false,
        "notes": "The Ashen Consortium has asked after it."
      },
      {
        "valuable_id": "val_02",
        "name": "Silver trade bar",
        "category": "trade_bar",
        "quantity": 3,
        "estimated_value_gp": 5,
        "weight_lb_total": 15,
        "inventory_entry_id": "inv_023",
        "is_sold": false
      }
    ],

    "starting_wealth": {
      "method": "background_package",
      "package_taken": true,
      "gold_alternative_gp": 50,
      "class_starting_gold_gp": null
    },

    "party_treasury_ref": {
      "party_treasury_id": "treas_camp44b1",
      "snapshot": { "gp": 640, "share_basis": "equal", "as_of": "2026-08-09T23:10:00Z" },
      "is_authoritative": false
    },

    "recurring_expenses": [
      { "expense_id": "exp_01", "label": "Bastion upkeep", "amount_gp": 25, "cadence": "per_bastion_turn", "active": true },
      { "expense_id": "exp_02", "label": "Lifestyle (comfortable)", "amount_gp": 2, "cadence": "per_day", "active": true }
    ],

    "ledger": [
      {
        "seq": 214,
        "at": "2026-08-09T22:40:00Z",
        "kind": "income",
        "delta": { "gp": 75 },
        "balance_after": { "cp": 46, "sp": 12, "ep": 0, "gp": 118, "pp": 4 },
        "counterparty": "Emerald Enclave",
        "reason": "Bounty: blightwood culling",
        "linked_reputation_event_id": "rep_ev_09",
        "session_number": 21
      },
      {
        "seq": 213,
        "at": "2026-08-09T14:05:00Z",
        "kind": "expense",
        "delta": { "gp": -50 },
        "balance_after": { "cp": 46, "sp": 12, "ep": 0, "gp": 43, "pp": 4 },
        "counterparty": "Herbalist, Greenrest",
        "reason": "Components for Potion of Greater Healing",
        "linked_item_instance_id": "itm_c118",
        "session_number": 21
      }
    ]
  }
}
```

---

## Part 6 — Reputation

### 6.1 What the rules actually say

Renown is an **optional rule in the 2024 DMG's toolbox chapter**, and its specifics are worth encoding exactly because they're more concrete than most people remember:

- Renown tracks a character's *or the party's* standing with a group — faction, organization, or community. A Renown Score starts at 0 and rises with favor earned.
- **Renown is tracked separately per group.** A character might sit at 5 with one faction and 20 with another.
- **Gaining:** advancing a group's interests is +1; completing a mission the group assigned or that directly benefits them is +2; hugely significant quests may be +3 or +4 at the DM's call. Separately, once a character is at 1+, downtime spent on minor tasks and socializing raises the score by 1 after a number of days equal to **10 times the current Renown Score** — an escalating cost curve the tool should compute and display.
- **Benefit thresholds:** at 3+ a character is a respected member, other members default to Friendly, and lodging and food are provided in dire circumstances; perks at 3+ might include a contact, a safe house, or gear discounts; at 10+, access to potions and scrolls, calling in a favor, or backup on missions; at 50, calling on a small army, a rare magic item, access to a spellcaster, or assigning missions to lower-ranked members. Ranks and titles are DM-defined thresholds layered on top.
- **Losing:** disagreements don't cost renown, but serious offenses do, by DM discretion. **A Renown Score can never drop below 0.** That floor is a hard validation constraint.
- **Level-based renown** is an alternative that skips score-tracking entirely, mapping character level to an equivalent score: level 1→1, 3→3, 5→10, 11→25, 17→50.

Renown is not the whole reputation surface. The DMG toolbox also carries **Marks of Prestige** (titles, letters of recommendation, medals, land grants, strongholds) and **Supernatural Gifts**, both of which are standing-shaped rewards without a numeric score. Beyond that, tables track individual NPC attitude (Hostile / Indifferent / Friendly is the codified social-interaction ladder), and settings add parallel systems like Piety.

### 6.2 Schema argument

**Reputation is per-(character, group) and must not be a scalar on the character.** That's the clearest structural implication of the rules text.

**Store the score, the rank, the unlocked perks, and the event ledger — all four, redundantly.** The perks are *derivable* from the score via thresholds, but the thresholds are DM-configurable, the perks are DM-invented, and a perk once granted shouldn't silently vanish if the score dips. Storing granted perks explicitly, each with the score at which it was granted, is both non-DRY and correct.

**Support both tracking modes as a per-group mode flag.** A group using level-based renown has no score to increment; the tool should display the mapped equivalent and disable the +1/+2 controls rather than pretending.

**Model the downtime progress bar explicitly.** The 10×score-days rule is exactly the kind of bookkeeping a tool should own: store `downtime_days_accumulated` and `downtime_days_required`, recompute the requirement when the score changes, and carry over or reset per the DM's ruling (store which).

**Keep numeric renown separate from qualitative standing.** A character can be at renown 12 with the Harpers and simultaneously *personally* hated by one Harper commander. Give groups a `default_attitude` and give individual NPCs their own standing entries.

**Ledger everything, with links.** A renown event that also paid 75 gp should reference the currency ledger entry. Cross-linking the two logs is duplicated relational information; it's also how the tool reconstructs a session.

```json
{
  "reputation": {
    "mode_default": "score",

    "groups": [
      {
        "group_id": "grp_emerald_enclave",
        "group_name": "Emerald Enclave",
        "group_kind": "faction",
        "is_member": true,
        "joined_at": "2026-05-02T00:00:00Z",
        "tracking_mode": "score",

        "renown_score": 12,
        "renown_score_minimum": 0,
        "renown_score_maximum": 50,
        "is_party_shared_score": false,
        "party_renown_score": null,

        "level_based_equivalent": null,

        "rank": {
          "current_rank_name": "Springwarden",
          "current_rank_index": 2,
          "rank_ladder": [
            { "index": 0, "name": "Sprout", "renown_required": 0 },
            { "index": 1, "name": "Greenwarden", "renown_required": 3 },
            { "index": 2, "name": "Springwarden", "renown_required": 10 },
            { "index": 3, "name": "Master of the Hunt", "renown_required": 25 }
          ],
          "promoted_at": "2026-08-02T00:00:00Z",
          "additional_prerequisites_note": "DM required completion of the Thornwatch trial."
        },

        "default_attitude": "friendly",
        "recognition_achieved": true,

        "perks_granted": [
          { "perk_id": "prk_01", "name": "Safe house access (Greenrest)", "granted_at_score": 3, "granted_at": "2026-05-28T00:00:00Z", "active": true, "revoked_at": null },
          { "perk_id": "prk_02", "name": "Potions and scrolls at cost", "granted_at_score": 10, "granted_at": "2026-08-02T00:00:00Z", "active": true, "revoked_at": null }
        ],

        "downtime_progress": {
          "enabled": true,
          "days_accumulated": 40,
          "days_required": 120,
          "formula_snapshot": "10 x current Renown Score",
          "resets_on_score_increase": true,
          "last_updated": "2026-08-06T00:00:00Z"
        },

        "contacts": [
          { "contact_id": "ct_01", "name": "Aldreth Fen", "role": "Quartermaster", "relationship": "ally", "notes": "Owes the party a favor." }
        ],

        "notes": "The Enclave is uneasy about the ruby.",

        "ledger": [
          {
            "event_id": "rep_ev_09",
            "at": "2026-08-09T22:40:00Z",
            "session_number": 21,
            "kind": "gain",
            "delta": 2,
            "score_after": 12,
            "reason_category": "assigned_mission",
            "reason": "Cleared the blightwood at the Enclave's request.",
            "awarded_by": "DM",
            "linked_currency_ledger_seq": 214
          },
          {
            "event_id": "rep_ev_08",
            "at": "2026-07-19T21:00:00Z",
            "session_number": 18,
            "kind": "gain",
            "delta": 1,
            "score_after": 10,
            "reason_category": "advanced_interests",
            "reason": "Rerouted the logging concession.",
            "awarded_by": "DM"
          }
        ]
      },
      {
        "group_id": "grp_ashen_consortium",
        "group_name": "The Ashen Consortium",
        "group_kind": "guild",
        "is_member": false,
        "tracking_mode": "level_based",
        "renown_score": null,
        "level_based_equivalent": { "character_level": 7, "mapped_renown_score": 10, "mapping_table_snapshot": [[1,1],[3,3],[5,10],[11,25],[17,50]] },
        "default_attitude": "indifferent",
        "recognition_achieved": true,
        "perks_granted": [],
        "downtime_progress": { "enabled": false },
        "ledger": []
      }
    ],

    "individual_standing": [
      {
        "npc_id": "npc_karrow",
        "npc_name": "Commander Karrow",
        "affiliated_group_id": "grp_emerald_enclave",
        "attitude": "hostile",
        "attitude_history": [
          { "at": "2026-06-14T00:00:00Z", "attitude": "indifferent", "reason": "First meeting." },
          { "at": "2026-07-30T00:00:00Z", "attitude": "hostile", "reason": "Publicly contradicted him at the Moot." }
        ],
        "notes": "Will not be swayed by renown alone."
      }
    ],

    "marks_of_prestige": [
      {
        "mark_id": "mrk_01",
        "kind": "title",
        "name": "Warden of the Thornwatch",
        "granted_by_group_id": "grp_emerald_enclave",
        "granted_at": "2026-08-02T00:00:00Z",
        "mechanical_effect_snapshot": null,
        "description": "Ceremonial; grants right of passage through Enclave lands.",
        "linked_item_instance_id": null
      },
      {
        "mark_id": "mrk_02",
        "kind": "land_grant",
        "name": "Deed to the Old Weir",
        "granted_by_group_id": "grp_emerald_enclave",
        "granted_at": "2026-08-02T00:00:00Z",
        "linked_bastion_id": "bst_01",
        "description": "Twelve acres and a ruined mill."
      }
    ],

    "alternate_tracks": [
      {
        "track_id": "trk_piety",
        "track_name": "Piety (Selûne)",
        "track_kind": "custom",
        "value": 8,
        "thresholds": [{ "at": 3, "label": "Devoted" }, { "at": 10, "label": "Favored" }],
        "notes": "Table-specific system; not from the 2024 core rules."
      }
    ]
  }
}
```

---

## Part 7 — The schema argument, consolidated

### 7.1 One aggregate root, catalogs as seed data

The character document is the unit of consistency. Everything needed to render and resolve a sheet lives inside it: species traits, feats, features, spells with mechanical fields, item instances (either embedded or in a per-character `item_instance` collection keyed by `owner_character_id`), currency, and reputation.

Catalog collections (`spell_catalog`, `item_catalog`, `species_catalog`, `background_catalog`, `feat_catalog`, `class_catalog`) exist to populate the picker UI and to seed new snapshots. They are **not joined at read time**. Every embedded object carries a `source_ref` recording where it came from, whether it's homebrew, and whether it's SRD-licensed — the last flag being load-bearing given the SRD 5.2 licensing boundary.

The honest counterargument: this makes global content updates hard. If errata changes Cure Wounds, no existing character updates automatically. My answer is that this is correct behavior, and the fix is an explicit, user-consented "your DM's ruleset updated — review 3 changes" migration flow, with the `source_ref.catalog_id` and a `snapshot_version` making the diff computable. Silent mutation of a live sheet is worse than a stale sheet.

### 7.2 Redundancy rules, stated as policy

| Redundancy | Justification |
|---|---|
| Raw inputs *and* computed totals (ability scores, AC, weight, currency value) | Sheets are explanations, not just numbers; every consumer would otherwise reimplement the math |
| Full text snapshot *and* catalog reference | Immutability of a played character; homebrew has no catalog row |
| Append-only logs *and* current state (build, inventory, currency, renown) | Undo, audit, "where did it go," DM disputes |
| Container parent pointer *and* denormalized location path | Moves need the pointer; rendering and orphan-recovery need the path |
| Granted perks *and* score thresholds | Thresholds are DM-configurable; granted perks shouldn't retroactively vanish |
| Party treasury reference *and* cached snapshot | Concurrency demands one owner; the UI still needs a number |

### 7.3 Discriminated unions over generic bags

Items use `item_type` as a discriminant with per-type sub-objects (`weapon`, `armor`, `consumable`, `container`) that are `null` when inapplicable. Spellcasting sources use `source_kind`. Reputation groups use `tracking_mode`. Each variant spells out its fields explicitly rather than sharing an `attributes` map. This is verbose and it is validatable, which a bag is not.

### 7.4 Identity and concurrency

- Every document has `_id`; every embedded object that can be mutated independently has a stable `instance_id` or `entry_id`. Never key an embedded object by array index or by name — names get changed by players.
- `doc_revision` supports optimistic concurrency. A shared table with a DM editing simultaneously will hit conflicts; the log-based sub-structures (ledgers) are append-only and merge cleanly, which is a quiet argument for having them.
- `ruleset.revision` on every document, because a table will mix 2014 and 2024 characters and the same field name means different things across them.

### 7.5 What I deliberately did *not* do

- No cross-character normalization of shared items (party loot copied per character, with a `party_shared` storage location instead).
- No single-unit currency normalization.
- No lookup-table extraction for repeated enums (damage types, schools, conditions) — they're stored as strings on each document, validated by schema `enum` rather than by a foreign key.
- No computed-on-read anything. Every derived value is stored with a `computed_at`.
- No index or shard-key design, per your instruction. When that becomes relevant, the obvious first questions are whether `campaign_id` or `owner_user_id` is the partition key and whether item instances stay embedded or split into a sibling collection — both of which are decisions this schema deliberately leaves open, since the character document works either way.

### 7.6 Open questions worth a decision before implementation

1. **Embedded vs. sibling item instances.** I showed items as separate documents with `owner_character_id`. Embedding them fully into the character document is equally valid and more self-contained; the tipping point is whether a character can plausibly carry hundreds of item instances (hoarders can) and whether a document size cap is in play.
2. **How much spell text to snapshot.** Full text is safest; mechanical-fields-only plus reference is the pragmatic compromise.
3. **Party-level documents.** Party treasury, shared loot, and party-level renown all want a `party` aggregate. This report keeps them as references; a fuller design would give the party its own root.
4. **Homebrew authoring.** The `source_ref.is_homebrew` and `authored_by_user_id` fields imply a homebrew content pipeline that this report doesn't specify.
5. **Rest and dawn semantics.** Long rest, short rest, and dawn are three distinct recharge triggers, and dawn is not a rest. The `rest_state` block handles this, but the resolution order at a table ("we long rest overnight") needs a stated rule.

---

## Sources consulted

- Wizards of the Coast / D&D Beyond: *Updates in the Player's Handbook (2024)*; *The 10 Species in the 2024 Player's Handbook*; *The Backgrounds and Origin Feats in the 2024 Player's Handbook*; *Your Guide to Weapon Mastery*; *4 Key Changes to Spells in the 2024 Player's Handbook*; *You Can Now Publish Your Own Creations Using the New Core Rules* (SRD 5.2 / CC-BY-4.0); D&D Beyond Basic Rules (2024) equipment and rules glossary; Potion of Healing item entry and rules forum thread on potion usage.
- Roll20 Compendium, *D&D 2024* (Dungeon Master's Guide 2024): Renown; Coins; Magic Items; Conditions; Rules Definitions; Casting Spells. Roll20 guides to 2024 Weapon Mastery and 2024 Backgrounds.
- Secondary rules coverage: RPGBot 2024 transition guide and weapon mastery guide; Arcane Eye backgrounds and healing-potion guides; Wargamer 2024 feats and 2024 weapons; EN World changelog and rules discussion threads; Kassoon 2024 weapons reference; ScreenRant and ComicBook coverage of SRD 5.2 and the prepared-spellcasting change.

*This report paraphrases rules mechanics for design purposes; it does not reproduce rulebook text. Shipping rule content in a product should be scoped to SRD 5.2.1 under CC-BY-4.0 or entered by users.*