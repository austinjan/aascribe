# aascribe AI Output Shapes

**Status:** Draft v1  
**Related docs:** [USAGE.md](USAGE.md)  
**Purpose:** Define how `OutputShape` values for `aascribe` commands map to stable machine-readable schemas for the `data` payload returned by the CLI.

---

## 1. Why this document exists

`aascribe` already documents a common JSON envelope in [USAGE.md](USAGE.md), but the command-level `data` payloads are still described ad hoc in per-command examples.

This document defines a reusable output-shape model so `aascribe` can:

1. give each command a stable symbolic result type
2. reuse shapes across commands where the logical data is the same
3. evolve command payloads without breaking AI clients unexpectedly

---

## 2. Terms

- **OutputShape**
  A symbolic, stable name attached to a command result, such as `MemoryEntryList` or `StoreInitResult`.
- **Shape schema**
  A JSON Schema fragment describing one `OutputShape`.
- **Envelope**
  The top-level response object returned by `aascribe`. `OutputShape` describes only the `data` field, not the full envelope.
- **Shape version**
  A version marker used only when a shape changes incompatibly.

---

## 3. Scope

This document defines:

- shape naming rules
- shape stability rules
- minimum schema expectations
- recommended first output shapes for `aascribe`
- how commands should map to shapes

This document does **not** define:

- the top-level JSON envelope itself
- CLI text rendering
- storage backend details
- ranking, summarization, or embedding internals

---

## 4. Shape model

Every concrete `aascribe` command should declare:

```rust
struct CommandSpec {
    path: &'static str,
    output_shape: &'static str,
    output_schema_ref: Option<&'static str>,
}
```

Interpretation:

- `output_shape` is the command's public symbolic result type.
- `output_schema_ref` points to the concrete schema entry if it differs from `output_shape`.
- In the common case, `output_schema_ref == output_shape`.
- Commands that return the same logical `data` structure should reuse the same `output_shape`.

Example:

```rust
CommandSpec {
    path: "recall",
    output_shape: "MemoryRecallResult",
    output_schema_ref: None,
}
```

Manifest-style schema excerpt:

```json
{
  "shapes": {
    "MemoryRecallResult": {
      "type": "object",
      "additionalProperties": false,
      "properties": {
        "query": { "type": "string" },
        "results": {
          "type": "array",
          "items": { "$ref": "#/shapes/MemoryRecallHit" }
        }
      },
      "required": ["query", "results"]
    }
  }
}
```

---

## 5. Normative rules

### 5.1 What `OutputShape` describes

- `OutputShape` describes the structure of the envelope's `data` field only.
- Envelope-level fields such as `ok`, `error`, `meta`, exit status handling, and text-mode rendering are outside the shape contract.

### 5.2 Shape naming

- Shape names must be stable symbolic identifiers in `PascalCase`.
- Good examples:
  - `StoreInitResult`
  - `PathIndexTree`
  - `FileDescription`
  - `MemoryRecallResult`
  - `MemoryEntryList`
- Avoid names tied too tightly to one exact command string if the same logical data may be reused elsewhere.
- Avoid version suffixes unless an incompatible shape change has already happened.

### 5.3 Shape reuse

- If two commands return the same logical structure, they must reuse the same `OutputShape`.
- If two commands differ only by filtering, scope, or ranking, they should usually share a shape.
- Do not create a new shape just because one command returns fewer items.

### 5.4 Shape stability

- Adding optional fields is backward-compatible.
- Removing fields, renaming fields, or changing field types is a breaking change.
- Breaking changes require either:
  - a new shape name such as `MemoryEntryListV2`, or
  - explicit version negotiation if `aascribe` later adds that machinery.
- For v1, prefer new shape names over in-place breaking changes.

### 5.5 Minimum schema quality

Every shape schema should define:

- `type`
- `properties` for object types
- `items` for array types
- `required` for fields consumers may rely on
- `description` when field meaning is not obvious

Preferred when known:

- `enum`
- `format`
- `pattern`
- `additionalProperties`
- nested `$ref` reuse for shared entities

### 5.6 Legacy compatibility

- `aascribe` does not need to normalize every command result in one big-bang change.
- A command may initially publish an `OutputShape` that matches its current payload as documented today.
- Later cleanup may consolidate or refactor shapes, but compatibility must be preserved or a new shape name must be introduced.

---

## 6. Recommended first shape set

The first implementation should keep the catalog small and reusable.

### Entity-level shapes

- `IndexedPathNode`
  One file or directory node in an indexed tree, including nested children for directory nodes.
- `MemoryEntrySummary`
  A compact memory record used in list-style results.
- `MemoryEntryDetail`
  A full single-memory representation for `show`.
- `MemoryRecallHit`
  One ranked result from a recall query.
- `ConsolidatedMemory`
  One long-term memory item created by `consolidate`.

### Collection and document shapes

- `PathIndexTree`
  Root object for `index` results.
- `MemoryEntryList`
  List wrapper for memory entries with count and items.
- `MemoryRecallResult`
  Recall response object containing query metadata and ranked hits.
- `ConsolidationResult`
  Consolidation response containing created items plus counts.

### Status and operation shapes

- `StoreInitResult`
  Result of initializing or reinitializing a store.
- `FileDescription`
  Single-file summary payload for `describe`.
- `RememberResult`
  Result of writing a short-term memory entry.
- `ForgetResult`
  Result of deleting a memory entry.

---

## 7. Shape design patterns

### 7.1 Prefer object wrappers over naked arrays

Prefer:

```json
{
  "count": 2,
  "items": [
    { "id": "stm_01", "content": "retry focas calls" },
    { "id": "stm_02", "content": "split poll loop" }
  ]
}
```

Over:

```json
[
  { "id": "stm_01", "content": "retry focas calls" },
  { "id": "stm_02", "content": "split poll loop" }
]
```

Why:

- easier to extend compatibly
- easier to include counts, cursors, or query metadata
- more stable for AI clients and generated code

### 7.2 Separate entity and collection shapes

Prefer:

- `MemoryEntrySummary`
- `MemoryEntryList`

Instead of repeating the same inline entry schema inside every list-style command.

### 7.3 Keep machine fields explicit

Prefer normalized fields like:

- `count`
- `items`
- `status`
- `score`
- `created_at`
- `stored_at`

Avoid embedding important machine state inside prose strings.

### 7.4 Keep human summary fields separate from raw content

Where payloads include both concise summaries and original memory text, keep them in distinct fields.

Examples:

- `content`
- `summary`
- `source`
- `tags`

This keeps AI consumers from having to parse semantics back out of prose.

### 7.5 Prefer stable wrappers for recursive data

`index` should expose a stable root object such as `PathIndexTree`, even though the underlying `IndexedPathNode` is recursive.

This makes it easier to add root-level fields later, such as:

- `root`
- `indexed_at`
- `truncated`

without changing the meaning of the recursive node type.

---

## 8. Command-to-shape examples

These examples are illustrative. They do not claim the current implementation already returns these exact structures.

| Command | OutputShape | Notes |
|---|---|---|
| `init` | `StoreInitResult` | Store creation or reinitialization result. |
| `index <path>` | `PathIndexTree` | Indexed directory tree and file summaries. |
| `describe <file>` | `FileDescription` | Single file summary payload. |
| `remember` | `RememberResult` | Newly stored short-term memory metadata. |
| `consolidate` | `ConsolidationResult` | Long-term memories created and source-entry counts. |
| `recall <query>` | `MemoryRecallResult` | Query plus ranked memory hits. |
| `list` | `MemoryEntryList` | Filtered memory rows for inspection. |
| `show <id>` | `MemoryEntryDetail` | Full raw memory detail. |
| `forget <id>` | `ForgetResult` | Deletion result for one memory entry. |

---

## 9. Example shape fragments

### 9.1 `MemoryEntrySummary`

```json
{
  "type": "object",
  "additionalProperties": false,
  "properties": {
    "id": { "type": "string" },
    "tier": { "type": "string", "enum": ["short", "long"] },
    "content": { "type": "string" },
    "tags": {
      "type": "array",
      "items": { "type": "string" }
    },
    "created_at": { "type": "string", "format": "date-time" }
  },
  "required": ["id", "tier", "content", "created_at"]
}
```

### 9.2 `MemoryEntryList`

```json
{
  "type": "object",
  "additionalProperties": false,
  "properties": {
    "count": { "type": "integer", "minimum": 0 },
    "items": {
      "type": "array",
      "items": { "$ref": "#/shapes/MemoryEntrySummary" }
    }
  },
  "required": ["count", "items"]
}
```

### 9.3 `StoreInitResult`

```json
{
  "type": "object",
  "additionalProperties": false,
  "properties": {
    "store": { "type": "string" },
    "created": { "type": "boolean" },
    "reinitialized": { "type": "boolean" },
    "layout_version": { "type": "string" }
  },
  "required": ["store", "created", "reinitialized"]
}
```

### 9.4 `MemoryRecallResult`

```json
{
  "type": "object",
  "additionalProperties": false,
  "properties": {
    "query": { "type": "string" },
    "results": {
      "type": "array",
      "items": { "$ref": "#/shapes/MemoryRecallHit" }
    }
  },
  "required": ["query", "results"]
}
```

---

## 10. Implementation guidance

### 10.1 Initial rollout

Start with:

1. define a small core shape catalog for the documented commands
2. attach `OutputShape` to each concrete CLI command
3. keep schema definitions checked in as JSON Schema fragments or derive them from typed Rust structs
4. add tests ensuring every user-facing command has a valid `OutputShape`

### 10.2 Validation

Recommended checks:

- every concrete command has a non-empty `OutputShape`
- every `OutputShape` resolves to a schema entry
- shared schemas are referenced by at least one command unless explicitly internal
- breaking changes to checked-in shape schemas require review
- documented output examples in [USAGE.md](USAGE.md) do not drift from the declared shapes

### 10.3 Ownership

- CLI/spec owners define which command maps to which shape
- implementation owners ensure actual `data` payloads conform to the declared schema
- CI or tests enforce that command mappings and schemas do not drift

---

## 11. Open decisions

The following are still implementation choices, not settled by this document:

- whether schemas are authored directly as JSON Schema files or generated from Rust types
- whether schema references use `#/shapes/...` or another stable path convention
- whether `list` and `recall` should share more underlying entity shapes beyond the obvious memory-entry fields
- whether `StoreInitResult` should expose layout details such as created directories or keep that internal

These should be decided before the first output-shape-aware implementation lands.
