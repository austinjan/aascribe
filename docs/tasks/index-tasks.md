# Index Implementation Tasks

Task breakdown for implementing `aascribe index` in the current Go codebase.

This document is execution-oriented. It turns the design ideas in [../INDEX_FUNCTION_MIGRATION_PLAN.md](../INDEX_FUNCTION_MIGRATION_PLAN.md) into concrete work items for `aascribe`, using the current project constraints:

- Go is the implementation language
- the CLI/output contract in [../USAGE.md](../USAGE.md) must remain the source of truth
- the `PathIndexTree` / `IndexedPathNode` schemas in [../shapes](../shapes/README.md) must be respected
- the LLM subsystem should exist before real file summarization is wired in

## Scope And Sequencing

Recommended delivery order:

1. LLM foundation
2. file extraction + summarize pipeline
3. `describe`
4. `index` core engine
5. incremental metadata reuse
6. recursive folder summaries

`index` should not be implemented as a one-shot file walker. The first useful version must already preserve the key system properties:

- folder-level indexing
- local metadata-based incremental updates
- bottom-up recursion
- partial failure tolerance

## Phase 0: Prerequisites

### Task 0.1: Finish the Gemini-backed LLM interface

- Implement the plan in [../plans/llm-interface-plan.md](../plans/llm-interface-plan.md)
- Follow the config strategy in [../archieved/config-strategy-tasks.md](../archieved/config-strategy-tasks.md)
- Deliver:
  - store-local config loading from `<store>/config.toml`
  - Gemini API key loading via `llm.api_key_env`
  - Gemini client
  - normalized summarizer interface
  - mocked HTTP tests

### Task 0.1a: Load and validate the Gemini API key

- Read the secret env-var name from `<store>/config.toml`
- Expected config shape:

```toml
[llm]
provider = "gemini"
model = "gemini-2.5-flash"
api_key_env = "GEMINI_API_KEY"
timeout_seconds = 30
```

- MVP rule:
  - load the actual API key from the env var named by `llm.api_key_env`
  - do not silently fall back to unrelated environment variables
- Validation behavior:
  - missing config file -> config-not-found error
  - missing `[llm]` section -> invalid-config error
  - missing `api_key_env` -> invalid-config error
  - referenced env var missing or empty -> missing-secret error
  - unsupported provider -> invalid-config error
- Pass the validated API key into Gemini client construction, not into command handlers directly
- Cover with tests for:
  - valid API key load
  - missing `api_key_env`
  - missing or empty referenced env var
  - wrong provider

### Task 0.2: Add a file extraction layer

- Create a reusable extraction module for reading file content before summarization
- Handle:
  - UTF-8 text files
  - oversize files
  - obviously binary files
  - extraction errors as typed results
- Keep extraction separate from `index` command code

### Task 0.3: Implement `describe` first

- Use the extraction layer plus LLM summarizer
- Match the `FileDescription` schema
- This becomes the first real consumer and proving ground for summarization behavior

## Phase 1: Index Engine Foundation

### Task 1.1: Add index packages

Create internal packages or subpackages for:

- index options
- metadata models
- filesystem scan
- metadata compare
- index execution
- summary assembly

Keep `internal/command` thin. It should call an index service, not contain indexing logic.

### Task 1.2: Define `index` runtime options

Translate CLI flags into a dedicated options struct with fields for:

- root path
- max depth
- include globs
- exclude globs
- refresh
- no-summary
- max file size
- recursive behavior

This struct should become the stable engine entrypoint for both `index` and future background/index scheduler work.

### Task 1.3: Define local metadata schema

Add a per-folder metadata file:

- filename: `.aascribe_index_meta.json`

Minimum metadata fields:

- version
- folder path
- folder hash
- last updated
- file records
- subdir tree
- folder description
- brief summary
- stats

Each file record should support at least:

- path
- content hash
- size
- mod time
- summary
- file type
- extraction cache path if used later
- error state

## Phase 2: Filesystem Scan And Change Detection

### Task 2.1: Implement immediate-folder scan

For one folder, gather:

- direct child files
- immediate subdirectories
- file size
- mod time
- content hash for eligible files

Do not mix recursive traversal into the single-folder scanner.

### Task 2.2: Implement include/exclude filtering

- Honor CLI include/exclude patterns before files enter expensive processing
- Apply the same filtering rules consistently across scan, compare, and output tree building
- Decide and document hidden-file behavior explicitly in code comments/tests

### Task 2.3: Implement metadata comparison

Produce a comparison result with:

- new files
- modified files
- deleted files
- unchanged files
- subdirectory-set changes
- needs-update flag

Special rules:

- previously failed files must be retried even when content hash is unchanged
- `--refresh` forces all eligible files into the changed set

## Phase 3: Per-File Processing

### Task 3.1: Reuse `describe`-level summarization logic

- `index` should not invent a separate file-summary pipeline
- Reuse the same extraction + summarization service used by `describe`
- Keep one consistent definition of summary length/behavior

### Task 3.2: Process changed files only

Changed set includes:

- new files
- modified files
- previous failures
- all files when refresh is enabled

Unchanged files should reuse prior metadata summaries.

### Task 3.3: Preserve partial success

- One bad file must not fail the whole folder
- Store per-file error state in metadata
- Continue indexing remaining files

## Phase 4: Recursive Folder Assembly

### Task 4.1: Implement bottom-up recursion

- Recurse into child folders first
- Load child metadata after child processing completes
- Build the parent folder summary/tree from child metadata plus current folder file summaries

This is a non-negotiable invariant for `aascribe index`.

### Task 4.2: Build folder summaries

After per-file processing:

- collect current folder file summaries
- collect child folder brief/detailed summaries
- generate:
  - folder description
  - brief summary

LLM is preferred here once the interface exists, but a deterministic fallback should exist for failure cases.

### Task 4.3: Build output tree

Generate the runtime `PathIndexTree` result from the current metadata snapshot.

Ensure the returned structure conforms to:

- [../shapes/PathIndexTree.schema.json](../shapes/PathIndexTree.schema.json)
- [../shapes/IndexedPathNode.schema.json](../shapes/IndexedPathNode.schema.json)

## Phase 5: Metadata Persistence

### Task 5.1: Write metadata atomically

- Save `.aascribe_index_meta.json` safely
- Avoid corrupt partial writes
- Version the metadata schema from day one

### Task 5.2: Reuse unchanged data

When files are unchanged:

- carry forward prior summary
- carry forward prior file type
- carry forward prior cache/extraction metadata if present

### Task 5.3: Handle deletions

- Remove deleted files from the new metadata snapshot
- Recompute folder stats and folder hash after deletions

## Phase 6: Command Integration

### Task 6.1: Wire `index` into the Go command layer

- Replace the current `NOT_IMPLEMENTED` stub for `index`
- Map CLI flags into index options
- Route errors through the existing JSON/text envelope machinery

### Task 6.2: Match the documented user contract

`index` must honor:

- `--depth`
- `--include`
- `--exclude`
- `--refresh`
- `--no-summary`
- `--max-file-size`

Return shape must stay aligned with [../USAGE.md](../USAGE.md).

### Task 6.3: Decide on `index.md`

Current recommendation:

- Do not generate `index.md` in the first Go implementation
- First ship metadata + JSON output
- Add `index.md` later as a presentation/export layer

This keeps the engine focused and avoids mixing human rendering concerns into the first indexing milestone.

## Testing Tasks

### Task T1: Scan and compare tests

- new file detection
- modified file detection by content hash
- deleted file detection
- unchanged file reuse
- previous failure retry behavior

### Task T2: Recursion tests

- child folders process before parent
- parent metadata includes child summaries
- depth limits are respected

### Task T3: CLI contract tests

- `index` JSON envelope shape
- `PathIndexTree` structural conformity
- `--no-summary` behavior
- `--refresh` behavior
- include/exclude behavior

### Task T4: Failure tests

- unreadable file
- binary file
- summarizer failure on one file
- metadata write failure

## Acceptance Criteria

The first acceptable `index` implementation should:

- return a valid `PathIndexTree`
- persist `.aascribe_index_meta.json` locally per indexed folder
- reuse unchanged file summaries across runs
- retry previously failed files
- recurse bottom-up
- survive per-file failures without dropping the whole folder
- integrate with the Go CLI envelope/output system cleanly

## Out Of Scope For First Index Milestone

- background worker orchestration
- job queues
- live progress streaming
- websocket callbacks
- scheduler integration
- image/vision indexing
- multi-provider LLM support
- `index.md` generation
