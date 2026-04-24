# Index Implementation Tasks

Task breakdown for implementing `aascribe index` in the current Go codebase.

This document is execution-oriented. It turns the design ideas in [../reference/INDEX_FUNCTION_MIGRATION_PLAN.md](../reference/INDEX_FUNCTION_MIGRATION_PLAN.md) into concrete work items for `aascribe`, using the current project constraints:

- Go is the implementation language
- the CLI/output contract in [../USAGE.md](../USAGE.md) must remain the source of truth
- the `PathIndexTree` / `IndexedPathNode` schemas in [../shapes](../shapes/README.md) must be respected
- the LLM subsystem should exist before real file summarization is wired in

## Current Status

Done in the current repo:

- `index` command is implemented and returns `PathIndexTree`
- `map` command is implemented as a top-level command, with `index map` supported as an alias
- `describe` command is implemented and returns `FileDescription`
- `describe` prefers the configured Gemini path when store config + secret resolution succeed
- `describe` falls back to a deterministic local summary when LLM config is unavailable
- `index` traverses directories, honors `--depth`, `--include`, `--exclude`, `--no-summary`, and `--max-file-size`
- `index` writes one `.aascribe_index_meta.json` per visited folder
- `index` metadata is now scoped to the current folder instead of accumulating descendant file records into the root metadata file
- `index` treats `.gitignore` and `.aaignore` as folder-scoped blacklist exclude files during recursion
- file-type eligibility is content-based: text files are summarized based on file bytes, not filename extension
- `index` writes `.aascribe_index_meta.json` atomically and records `not_indexed_files`
- `map` assembles a hierarchy by reading local metadata files recursively instead of relying on child pointers stored inside metadata
- `map` omits ignored directories during traversal
- `map` now projects metadata into an LLM-facing assembled shape and offers a compact text-tree renderer
- `map` now exposes simple node states: `ready`, `dirty`, and `unindexed`
- the CLI default format is now compact `text`; `json` is explicitly requested when machine parsing is needed
- `index` now reuses unchanged direct-file metadata across runs
- `index` now supports bounded direct-file concurrency and a `--concurrency` CLI flag
- `index dirty` is implemented and marks existing per-folder metadata stale recursively
- `index eval` is implemented and reports which folders and direct files need indexing versus which are unchanged
- fixture-based tests exist under `tests/index-fixtures` and cover ignore behavior, text detection, binary detection, and multi-level traversal

Still intentionally incomplete:

- `index` still owns its own summary assembly instead of reusing one shared `describe` service end-to-end
- folder summaries are still deterministic placeholders, not real bottom-up LLM summaries
- `index` runs synchronously and does not yet provide agent-friendly handling for long-running indexing work
- recursive folder traversal is still serial; folder-level orchestration concurrency and shared folder semaphore control are not implemented yet

## Scope And Sequencing

Recommended delivery order:

1. finish one reusable file-analysis / summarization service shared by `describe` and `index`
2. upgrade `index` file summaries to reuse that service instead of its current independent summary path
3. narrow `index` metadata to a fully local-only schema for one folder at a time
4. add metadata comparison and unchanged-file reuse
5. add explicit metadata-control commands such as `index dirty` and `index eval`
6. add a separate hierarchy-map command that aggregates local metadata files into one full view
7. add bounded concurrency for file processing and folder orchestration
8. improve long-running `index` execution behavior for agent/CLI callers

`index` should not be implemented as a one-shot file walker. The first useful version must already preserve the key system properties:

- folder-level indexing
- local metadata-based incremental updates
- partial failure tolerance

## Responsibility Split

`index` and hierarchy-map generation are separate responsibilities and should remain separate in implementation and command surface.

`index` is responsible for:

- scanning one folder and its recursive children as needed to refresh local metadata files
- summarizing the current folder
- summarizing direct files in the current folder
- writing one `.aascribe_index_meta.json` per folder

`index` is not responsible for:

- storing a full descendant tree in one metadata file
- persisting a whole-workspace hierarchy map
- acting as the only read surface for agents that need a full tree view

The hierarchy-map command is responsible for:

- starting from one folder
- reading `.aascribe_index_meta.json` files from that folder and its child directories
- assembling a derived full hierarchy map without re-summarizing files
- surfacing missing, stale, or failed child metadata explicitly in the assembled map
- projecting local metadata into a more LLM-friendly output shape
- rendering a compact text tree for text output
- treating compact text as the default caller experience and JSON as an explicit machine mode

`index dirty` is responsible for:

- marking existing `.aascribe_index_meta.json` files under one folder tree as stale
- forcing later `index` runs to rebuild those folders even if direct files appear unchanged

`index eval` is responsible for:

- comparing current direct files against local per-folder metadata
- reporting which folders and direct files need indexing
- also reporting unchanged folders and files so agents can see the full local decision surface

## Phase 0: Prerequisites

### Task 0.1: Finish the Gemini-backed LLM interface

Status: mostly done for the current Gemini-only MVP.

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

Status: partially done inside `internal/index`, but not yet factored into a standalone reusable extraction module.

- Create a reusable extraction module for reading file content before summarization
- Handle:
  - UTF-8 text files
  - oversize files
  - obviously binary files
  - extraction errors as typed results
- Keep extraction separate from `index` command code

### Task 0.3: Implement `describe` first

Status: done as a command surface, but still needs cleanup into a more explicit shared service boundary.

- Use the extraction layer plus LLM summarizer
- Match the `FileDescription` schema
- This becomes the first real consumer and proving ground for summarization behavior

## Phase 1: Index Engine Foundation

### Task 1.1: Add index packages

Status: partially done.

Create internal packages or subpackages for:

- index options
- metadata models
- filesystem scan
- metadata compare
- index execution
- summary assembly

Keep `internal/command` thin. It should call an index service, not contain indexing logic.

### Task 1.2: Define `index` runtime options

Status: mostly done for the current CLI surface.

Translate CLI flags into a dedicated options struct with fields for:

- root path
- max depth
- include globs
- exclude globs
- refresh
- no-summary
- max file size
- recursive behavior derived from `--depth`

This struct should become the stable engine entrypoint for both `index` and future background/index scheduler work.

### Task 1.3: Define local metadata schema

Status: started, but minimal.

Add a per-folder metadata file:

- filename: `.aascribe_index_meta.json`

Current fields:

- version
- folder path
- last updated
- not-indexed files

Target additional fields:

- folder hash
- immediate file records only
- folder description
- brief summary
- stats

Progressive disclosure rules:

- each folder writes its own `.aascribe_index_meta.json`
- a folder metadata file must not inline full descendant file records from grandchildren or deeper paths
- a folder metadata file should be independently useful without requiring one root super-document
- runtime `PathIndexTree` output and persisted metadata remain separate concerns
- the full hierarchy map is a derived artifact built by a separate command, not the persistence format

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

Status: current tree walk is implemented, but not yet split into a dedicated immediate-folder scanner plus comparison phase.

For one folder, gather:

- direct child files
- immediate subdirectories
- file size
- mod time
- content hash for eligible files

Do not mix recursive traversal into the single-folder scanner.

The single-folder scanner should not append descendant file records into the current folder metadata.

### Task 2.2: Implement include/exclude filtering

Status: partial for the current engine.

- Honor CLI include/exclude patterns before files enter expensive processing
- Apply the same filtering rules consistently across scan, compare, and output tree building
- Treat `.gitignore` and `.aaignore` as blacklist excludes for the folder where they are defined
- Decide and document hidden-file behavior explicitly in code comments/tests

Current gap:

- only the indexed root ignore files are loaded today; nested ignore files are not reapplied while recursing

### Task 2.3: Implement metadata comparison

Status: not started.

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
- child-folder summary/hash changes must mark the parent folder as needing summary rebuild even when the parent's direct files are unchanged

## Phase 3: Per-File Processing

### Task 3.1: Reuse `describe`-level summarization logic

Status: partial.

- `index` should not invent a separate file-summary pipeline
- Reuse the same extraction + summarization service used by `describe`
- Keep one consistent definition of summary length/behavior

Current gap:

- `describe` and `index` both rely on the same low-level file analysis, but `index` does not yet call one explicit shared summarization service end-to-end
- `describe` can already use Gemini; `index` still uses deterministic local summaries

### Task 3.2: Process changed files only

Status: not started.

Changed set includes:

- new files
- modified files
- previous failures
- all files when refresh is enabled

Unchanged files should reuse prior metadata summaries.

Implementation note:

- changed-file processing should use bounded concurrency rather than serial processing or unbounded goroutines

### Task 3.3: Preserve partial success

Status: partial.

- One bad file must not fail the whole folder
- Store per-file error state in metadata
- Continue indexing remaining files

Current behavior:

- files that are not content-indexed are tracked in `not_indexed_files`
- structured per-file reusable error state is not implemented yet

## Phase 4: Recursive Folder Assembly

### Task 4.1: Implement bottom-up recursion

Status: traversal works recursively, but true bottom-up metadata-driven summary assembly is not implemented yet.

- Recurse into child folders first
- Load child metadata after child processing completes
- Build the parent folder summary/tree from child metadata plus current folder file summaries

This is a non-negotiable invariant for `aascribe index`.

Parent folders must not finalize metadata until all eligible child folders have either:

- produced fresh metadata, or
- surfaced a structured child error/stale state that the parent can record

### Task 4.2: Build folder summaries

Status: placeholder implementation exists.

After per-file processing:

- collect current folder file summaries
- collect child folder brief/detailed summaries
- generate:
  - folder description
  - brief summary

Current behavior:

- directory summaries are deterministic count-based placeholders
- LLM-backed bottom-up folder summaries remain future work

Desired child-folder summary shape:

- tell the LLM what kinds of files or topics are in the child folder
- stay compact enough for routing and drill-down decisions
- avoid copying all child file summaries into the parent metadata

Revision:

- keep rich child-folder description in the child folder's own metadata file
- keep parent metadata local-only and free of persisted child pointers
- let the separate hierarchy-map command read child directories and child metadata when a richer assembled view is needed

### Task 4.3: Build output tree

Status: done for the current schema.

Generate the runtime `PathIndexTree` result from the current metadata snapshot.

Ensure the returned structure conforms to:

- [../shapes/PathIndexTree.schema.json](../shapes/PathIndexTree.schema.json)
- [../shapes/IndexedPathNode.schema.json](../shapes/IndexedPathNode.schema.json)

## Phase 5: Metadata Persistence

### Task 5.1: Write metadata atomically

Status: done for the current minimal metadata file.

- Save `.aascribe_index_meta.json` safely
- Avoid corrupt partial writes
- Version the metadata schema from day one

### Task 5.2: Reuse unchanged data

Status: not started.

When files are unchanged:

- carry forward prior summary
- carry forward prior file type
- carry forward prior cache/extraction metadata if present
- carry forward prior child-folder summary data when the child metadata is unchanged

### Task 5.3: Handle deletions

Status: not started.

- Remove deleted files from the new metadata snapshot
- Recompute folder stats and folder hash after deletions

## Phase 6: Command Integration

Status: `index` and `describe` command surfaces are both implemented.

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

The command contract should also make long-running behavior explicit for agent callers:

- synchronous `index` is acceptable for small trees and explicit user requests
- larger or repeated index runs should be able to reuse metadata aggressively
- follow-up work may add a background or job-style surface, but engine correctness comes first

### Task 6.3: Add a hierarchy-map command

Status: partial.

Add a command, now implemented primarily as `map` with `index map` as an alias, that:

- accepts a start folder
- reads local `.aascribe_index_meta.json` files recursively
- assembles one derived hierarchy object for agents or callers that want a full map
- reports missing child metadata explicitly instead of silently reindexing
- does not re-summarize file contents as part of map assembly
- supports a compact text-tree output for LLM-facing usage

Recommended output contract:

- use `PathIndexMap`
- keep it distinct from `PathIndexTree`, which is the runtime result of `index`
- include node status for missing, stale, or failed child metadata
- project away low-signal persistence fields such as timestamps and hashes from the assembled map output

### Task 6.4: Decide on `index.md`

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

Concrete cases:

- index one folder, rerun without changes, and assert file summaries/hashes are reused from existing metadata
- change one direct file, rerun, and assert only that file record is re-summarized while unchanged siblings keep prior summary metadata
- delete one direct file, rerun, and assert it is removed from file records, stats, folder hash inputs, and runtime tree
- add one new direct file, rerun, and assert it appears in both runtime output and folder metadata without forcing unchanged siblings to rerun
- keep file bytes unchanged for a previously failed file, rerun, and assert the file is retried because prior status was failed
- run with `--refresh` and assert all eligible direct files are treated as changed even when hashes match prior metadata

### Task T2: Recursion tests

- child folders process before parent
- parent metadata includes child summaries
- parent metadata excludes grandchild file-detail expansion
- depth limits are respected

Concrete cases:

- index a three-level tree and assert parent summary timestamps are not earlier than child metadata used to build them
- modify a deep child folder summary input, rerun, and assert the immediate parent and root parent both rebuild their derived summaries
- index with `--depth=0` and assert only the root folder metadata is produced
- index with `--depth=1` and assert direct child folder metadata exists while deeper grandchildren metadata is absent
- index with `--depth=2` and assert grandchild folder metadata exists and parent metadata links to it through compact child summaries only

### Task T2a: Progressive disclosure metadata tests

- one metadata file per indexed folder
- folder metadata contains direct files only
- folder metadata contains compact child-folder summaries
- deep file details appear only in the matching child folder metadata

Concrete cases:

- root metadata lists `root/a.txt` and `root/docs` summary, but does not inline `root/docs/guide.md`
- child folder metadata for `root/docs` lists `guide.md` and `api.md`, while root metadata retains only the child folder summary/stats for `docs`
- grandchild metadata for `root/docs/reference` contains its own direct files and does not duplicate sibling folder file details
- child-folder entry includes enough traversal detail such as status, stats, or has-meta signal, not only raw child folder name
- metadata JSON schema or fixture assertions fail if any folder metadata file contains descendant file records deeper than one level
- runtime `PathIndexTree` may still contain recursive children, but persisted metadata must remain shallow per folder

### Task T2d: Hierarchy-map command tests

- reads local metadata files recursively and assembles one full map
- does not summarize or reopen source files when metadata is already present
- reports missing child metadata as structured missing-state nodes
- preserves child failure/stale states in the assembled output
- distinguishes local metadata shape from assembled hierarchy-map shape

### Task T2b: Concurrency tests

- bounded worker count is respected
- parallel file processing still yields deterministic metadata/output
- parent rebuild waits for child completion without deadlock

Concrete cases:

- use a blocking test summarizer and assert in-flight file summaries never exceed configured max concurrency
- process many sibling files with concurrency greater than 1 and assert resulting metadata order/output remains deterministic
- process many sibling folders with shared folder concurrency control and assert total active folder jobs stay within the configured limit
- run a deep tree with low concurrency and assert the traversal completes without ancestor-held semaphore deadlock
- inject one slow child folder and assert parent metadata is written only after that child finishes or records structured failure state

### Task T2c: Ignore inheritance tests

- nested `.gitignore` applies inside the nested folder
- nested `.aaignore` applies inside the nested folder
- parent/root ignore state does not incorrectly replace child-local ignore rules

Concrete cases:

- root `.gitignore` excludes `ignored-at-root.txt`, while nested folder `.gitignore` excludes only `child-only.txt`; assert each rule applies only in its own scope
- nested `.aaignore` excludes `*.skip` inside one child folder without removing matching files from sibling folders
- child folder with its own ignore file still inherits parent path exclusion behavior where applicable
- metadata and runtime tree both omit ignored entries; ignored files must not appear in file records, child summaries, or failure lists
- ignored descendant files must not be summarized before being filtered out

### Task T3: CLI contract tests

- `index` JSON envelope shape
- `PathIndexTree` structural conformity
- `--no-summary` behavior
- `--refresh` behavior
- include/exclude behavior

Concrete cases:

- CLI JSON mode returns a valid `PathIndexTree` envelope while metadata files remain on disk as separate artifacts
- `--no-summary` preserves traversal output but leaves summary fields empty and records non-indexed reason consistently in metadata
- `--refresh` forces reprocessing without changing the documented output envelope shape
- include/exclude filters affect both runtime output and persisted metadata consistently
- long-running index execution still returns one final command result without partial malformed JSON output
- hierarchy-map command returns the derived assembled shape without mutating metadata files

### Task T4: Failure tests

- unreadable file
- binary file
- summarizer failure on one file
- metadata write failure

Concrete cases:

- unreadable direct file is recorded as failed while sibling files still complete and folder metadata is still written
- unreadable child folder surfaces as child error/stale state in parent metadata without aborting the whole root index
- binary file is marked not indexed and does not enter summarizer pipeline
- summarizer failure on one changed file does not erase prior good metadata for unchanged siblings
- metadata write failure for one folder returns an error and does not leave a truncated `.aascribe_index_meta.json`
- child metadata corruption on rerun is either repaired by rebuild or surfaced as structured failure; it must not silently poison parent summaries

## Acceptance Criteria

The first acceptable `index` implementation should:

- return a valid `PathIndexTree`
- persist `.aascribe_index_meta.json` locally per indexed folder
- keep each metadata file scoped to the current folder with direct files plus compact child-folder pointers/status
- reuse unchanged file summaries across runs
- retry previously failed files
- recurse bottom-up
- support bounded concurrency without unbounded goroutine growth
- survive per-file failures without dropping the whole folder
- integrate with the Go CLI envelope/output system cleanly

The first acceptable hierarchy-map implementation should:

- start from one folder and read local metadata files recursively
- build a complete assembled hierarchy view without re-summarizing source files
- surface missing or stale child metadata explicitly
- keep assembled-map output distinct from per-folder metadata persistence

## Out Of Scope For First Index Milestone

- full background worker orchestration
- external job queues
- live progress streaming
- websocket callbacks
- scheduler integration
- image/vision indexing
- multi-provider LLM support
- `index.md` generation
