# aascribe Shape Schemas

This directory is the checked-in source of truth for `aascribe` command `OutputShape` schemas.

## Policy

- Each command documented in [../USAGE.md](../USAGE.md) declares an explicit `OutputShape`.
- Each declared shape must have a matching JSON Schema file in this directory.
- `OutputShape` describes the `data` field inside the top-level CLI JSON envelope, not the full envelope.
- Backward-incompatible shape changes should create a new shape name instead of silently rewriting an existing schema.

## Current Shape Files

- `ConsolidatedMemory.schema.json`
- `ConsolidationResult.schema.json`
- `FileDescription.schema.json`
- `ForgetResult.schema.json`
- `IndexedPathNode.schema.json`
- `LogClearResult.schema.json`
- `LogExportResult.schema.json`
- `LogPathResult.schema.json`
- `MemoryEntryDetail.schema.json`
- `MemoryEntryList.schema.json`
- `MemoryEntrySummary.schema.json`
- `MemoryRecallHit.schema.json`
- `MemoryRecallResult.schema.json`
- `PathIndexTree.schema.json`
- `RememberResult.schema.json`
- `StoreInitResult.schema.json`

## Notes

- Schemas are authored directly as checked-in JSON Schema files for now.
- If `aascribe` later generates schemas from Go types, those generated artifacts must still stay aligned with this directory or replace it explicitly as the new source of truth.
