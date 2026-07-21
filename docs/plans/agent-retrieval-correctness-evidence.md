# Agent Retrieval Correctness TDD Evidence

This log records the public-interface Red-Green-Regression evidence required by the execution plan. Each numbered slice is completed vertically before the next slice begins.

## Slice 1 — Read-only questions do not index

- Test: `TestSkillDefaultsRepositoryQuestionsToReadOnlyDiscovery` specifies that the skill's default repository-question workflow explicitly prohibits mutation and contains no `init` or write-index command.
- RED: `go test ./internal/app -run '^TestSkillDefaultsRepositoryQuestionsToReadOnlyDiscovery$' -count=1` failed because the old skill had no read-only default section and started with `init` and `index`.
- GREEN: the same focused command passed after separating the default read-only workflow from the explicitly mutating indexing workflow.
- Regression: `go test ./internal/app` and `go test ./...` passed.
- Contract changes: `skills/indexing-folder/SKILL.md` now defaults ordinary questions to read-only discovery, identifies index artifacts, and no longer demonstrates exporting a real API secret.

## Slice 2 — Known evidence bypasses semantic routing

- Test: `TestSkillRoutesKnownEvidenceWithoutRequiringMapFirst` specifies distinct public workflow branches for a known file, a known exact token, and uncertain terminology.
- RED: `go test ./internal/app -run '^TestSkillRoutesKnownEvidenceWithoutRequiringMapFirst$' -count=1` failed because the old skill had no intent-based routing section and described a mandatory map-then-search sequence.
- GREEN: the same focused command passed after adding intent-based routing and removing the linear-flow claim from the skill description.
- Regression: `go test ./internal/app` and `go test ./...` passed.
- Contract changes: known files route to direct inspection, known literals route to fixed-string search first, and map is reserved for semantic routing.

## Slice 33 — Absence claims require adequate evidence

- Test: `TestSkillRequiresCompleteUntruncatedScopeForAbsenceClaims` specifies the three preconditions for an absence claim: adequate declared scope, `complete: true`, and `truncated: false`.
- RED: `go test ./internal/app -run '^TestSkillRequiresCompleteUntruncatedScopeForAbsenceClaims$' -count=1` failed because the skill had no explicit absence-claim contract.
- GREEN: the same focused command passed after adding the required evidence gate and recovery guidance.
- Regression: `go test ./internal/app` and `go test ./...` passed.
- Contract changes: the skill and usage documentation prohibit absence claims from stale, partial, unreadable, narrower-scope, or truncated evidence. Current missing completeness metadata is documented as a limitation until Phase 2.

## Slice 21 — Empty search queries are invalid

- Test: `TestRunSearchRejectsEmptyQuery` invokes `app.Run` and specifies that an empty query returns the public `INVALID_ARGUMENTS` JSON contract with exit status 2.
- RED: `go test ./internal/app -run '^TestRunSearchRejectsEmptyQuery$' -count=1` returned success with every line matched. After query validation was added, the same test exposed that wrapped parse errors incorrectly exited with status 1.
- GREEN: the focused test passed after rejecting the empty positional query and normalizing wrapped application errors to their documented exit status.
- Regression: `go test ./internal/app` and `go test ./...` passed.
- Contract changes: empty queries are rejected at the CLI boundary; parse errors carrying `INVALID_ARGUMENTS` now consistently exit with status 2.

## Slice 22 — Explicit zero is not the default limit

- Test: `TestRunSearchHonorsExplicitZeroMaxCount` specifies that `--max-count 0` returns zero matching lines and reports truncation when matching content exists.
- RED: `go test ./internal/app -run '^TestRunSearchHonorsExplicitZeroMaxCount$' -count=1` returned two matches because runtime code rewrote zero to 100.
- GREEN: the same focused command passed after removing the runtime rewrite; the parser remains responsible for applying the default 100 only when the flag is unspecified.
- Regression: `go test ./internal/app` and `go test ./...` passed.
- Contract changes: explicit zero is honored consistently with existing help and usage documentation.

## Slice 23 — Exact N-line limit has explicit truncation semantics

- Test: `TestRunSearchReportsExactLimitAndTruncation` exercises JSON and text output for three matching lines with `--max-count 2`.
- RED: `go test ./internal/app -run '^TestRunSearchReportsExactLimitAndTruncation$' -count=1` returned exactly two lines and `truncated: true`, but lacked the required machine-readable returned-line count.
- GREEN: the same focused command passed after adding additive `returned_match_lines` output and text parity while retaining `match_count`.
- Regression: `go test ./internal/app` and `go test ./...` passed.
- Contract changes: `returned_match_lines` is the precise count; `match_count` remains a compatibility alias during migration.

## Slice 24 — Search results are matching lines, not occurrences

- Test: `TestRunSearchDeclaresMatchingLineUnitForRepeatedOccurrences` searches a line containing the same literal twice and requires one result with `match_unit: "line"`.
- RED: `go test ./internal/app -run '^TestRunSearchDeclaresMatchingLineUnitForRepeatedOccurrences$' -count=1` returned one line correctly but did not declare the unit, leaving exhaustive-occurrence interpretation ambiguous.
- GREEN: the same focused command passed after adding additive `match_unit: "line"` to JSON and text results.
- Regression: `go test ./internal/app` and `go test ./...` passed.
- Contract changes: one result represents one matching line; per-occurrence submatches are not claimed by this mode.

## Slice 25 — File scope never broadens to its parent

- Test: `TestRunSearchFileScopeDoesNotSearchSiblingFiles` passes one exact file whose sibling contains the only match.
- RED: `go test ./internal/app -run '^TestRunSearchFileScopeDoesNotSearchSiblingFiles$' -count=1` reported the parent directory as root and returned the sibling match.
- GREEN: the same focused command passed after preserving file roots and rendering the exact file basename instead of `.`.
- Regression: `go test ./internal/app` and `go test ./...` passed.
- Contract changes: a file path is an exact search scope and is never silently broadened.

## Slice 20 — Fixed-string metacharacters stay literal

- Test: `TestRunSearchFixedStringMetacharactersAreLiteralAcrossEngines` runs a regex-metacharacter literal and decoy through every locally available supported engine.
- RED: `go test ./internal/app -run '^TestRunSearchFixedStringMetacharactersAreLiteralAcrossEngines$' -count=1` showed equivalent one-line matching but every engine omitted the required query-mode declaration.
- GREEN: the same focused command passed after adding additive `query_mode: "fixed"` to JSON and text output.
- Regression: `go test ./internal/app` and `go test ./...` passed.
- Contract changes: callers can verify that metacharacters were interpreted literally instead of inferring mode from the request.

## Slice 17 — Fixed-string coverage includes tracked and untracked files

- Test: `TestRunSearchFixedStringCoverageIncludesTrackedAndUntrackedAcrossEngines` runs a shared Git-worktree fixture through every supported engine.
- RED: `go test ./internal/app -run '^TestRunSearchFixedStringCoverageIncludesTrackedAndUntrackedAcrossEngines$' -count=1` showed `git-grep` returning only the tracked match while builtin, rg, and grep returned both tracked and untracked matches.
- GREEN: the same focused command passed after selecting Git grep's `--no-index` worktree mode.
- Regression: `go test ./internal/app` and `go test ./...` passed.
- Contract changes: default fixed-string coverage is not tracked-only.

## Slice 18 — Every engine enforces and describes one default scope

- Test: `TestRunSearchEnforcesAndDescribesOneDefaultScopeAcrossEngines` combines tracked, untracked, hidden, gitignored, and aaignored files in one public CLI fixture.
- RED: `go test ./internal/app -run '^TestRunSearchEnforcesAndDescribesOneDefaultScopeAcrossEngines$' -count=1` produced five matches from builtin, grep, and git-grep, three from rg, and no scope metadata.
- GREEN: the same focused command passed after introducing a shared deterministic candidate-file projection consumed by every engine. A package regression exposed grep's single-file filename omission and was fixed with explicit filename output.
- Regression: `go test ./internal/app` and `go test ./...` passed.
- Contract changes: default scope includes tracked and untracked files, excludes hidden and ignored files, and applies the documented blacklist subset from `.gitignore` and `.aaignore`; these decisions are additive JSON scope fields.

## Slice 26 — Unreadable content makes search partial

- Test: `TestRunSearchUnreadableFileMakesResultIncomplete` combines one readable matching file with one permission-denied matching file.
- RED: `go test ./internal/app -run '^TestRunSearchUnreadableFileMakesResultIncomplete$' -count=1` returned the healthy match but silently omitted the unreadable file and exposed no completeness decision.
- GREEN: the same focused command passed after candidate discovery started collecting structured unreadable reasons and results added `complete` plus `partial_reasons` in JSON and text.
- Regression: `go test ./internal/app` and `go test ./...` passed.
- Contract changes: skipped unreadable files or subdirectories produce useful partial success with `complete: false`; root-scope failure remains a command error.

## Slice 27 — Supported long lines are searched

- Test: `TestRunSearchFindsMatchOnLongSupportedLine` puts a match near the end of a 128 KiB line and invokes builtin search through `app.Run`.
- RED: `go test ./internal/app -run '^TestRunSearchFindsMatchOnLongSupportedLine$' -count=1` silently returned zero matches with `complete: true` because Go Scanner stopped at its default token limit.
- GREEN: the same focused command passed after defining a 1 MiB supported token bound for builtin and external-output scanners; over-bound builtin lines now produce a `scanner_limit` partial reason.
- Regression: `go test ./internal/app` and `go test ./...` passed.
- Contract changes: lines up to 1 MiB are supported; scanner-limit failures make the result incomplete instead of becoming false zero results.

## Slice 28 — Binary exclusion is not an I/O failure

- Test: `TestRunSearchReportsBinaryExclusionWithoutIOFailure` combines one text match with a NUL-containing binary match candidate.
- RED: `go test ./internal/app -run '^TestRunSearchReportsBinaryExclusionWithoutIOFailure$' -count=1` silently skipped the binary file without distinguishing the deliberate exclusion.
- GREEN: the same focused command passed after common candidate inspection started recording binary exclusions before engine dispatch.
- Regression: `go test ./internal/app` and `go test ./...` passed.
- Contract changes: `exclusions` reports deliberate binary exclusions while `partial_reasons` is reserved for failures; binary exclusion alone does not make search incomplete.

## Slice 29 — Separator characters in filenames do not corrupt locations

- Test: `TestRunSearchPreservesColonInFilenameAcrossEngines` searches `colon:name.txt` through every locally available engine and asserts exact path, line, and column.
- RED: `go test ./internal/app -run '^TestRunSearchPreservesColonInFilenameAcrossEngines$' -count=1` showed grep and git-grep truncating the path to `colon` and corrupting numeric fields.
- GREEN: the same focused command passed after external line protocols switched from ambiguous colon parsing to NUL-delimited filenames, with Git's structured numeric fields handled explicitly.
- Regression: `go test ./internal/app` and `go test ./...` passed.
- Contract changes: filename contents cannot be confused with location separators.

## Slice 30 — Columns are one-based byte offsets

- Test: `TestRunSearchDeclaresByteColumnsAcrossEngines` places a match after two multibyte Chinese characters and runs every available engine.
- RED: `go test ./internal/app -run '^TestRunSearchDeclaresByteColumnsAcrossEngines$' -count=1` showed all engines agreeing on byte column 8 but omitting the unit declaration.
- GREEN: the same focused command passed after adding additive `column_unit: "byte"` to JSON and text output.
- Regression: `go test ./internal/app` and `go test ./...` passed.
- Contract changes: columns are documented as one-based byte offsets across supported engines.

## Slice 19 — Regex fallback cannot change dialect

- Test: `TestRunSearchRegexFallbackNeverChangesAlternationMeaning` constrains PATH to grep only, then checks auto fallback and explicit incompatible-engine behavior through `app.Run`.
- RED: `go test ./internal/app -run '^TestRunSearchRegexFallbackNeverChangesAlternationMeaning$' -count=1` selected grep and returned a false complete zero result for `foo|bar`.
- GREEN: the same focused command passed after regex auto fallback was restricted to compatible rg and builtin engines, and explicit grep/git-grep regex requests gained `UNSUPPORTED_QUERY` with exit status 2.
- Regression: `go test ./internal/app` and `go test ./...` passed.
- Contract changes: fixed strings support all engines; regex uses the compatible rg/builtin path and never silently falls back to basic grep syntax.

## Slice 31 — External output is bounded during production

- Test: `TestRunSearchStopsExternalOutputAfterLimitPlusOne` uses a fake rg that emits 5,000 valid events and writes a marker only if allowed to finish.
- RED: `go test ./internal/app -run '^TestRunSearchStopsExternalOutputAfterLimitPlusOne$' -count=1` returned one line with truncation but the marker existed, proving the full subprocess output was buffered before applying the limit.
- GREEN: the same focused command passed after rg output became streamed and the producer was stopped on the N+1 event.
- Refactor while green: the same streaming and early-stop protocol was applied to grep and git-grep's NUL parser.
- Regression: `go test ./internal/app` and `go test ./...` passed.
- Contract changes: external production is bounded by the requested global matching-line limit plus one evidence line.

## Slice 32 — External execution failure is classified

- Test: `TestRunSearchClassifiesExternalExecutionFailure` provides an rg executable that is detected and starts but exits 2 with controlled stderr.
- RED: `go test ./internal/app -run '^TestRunSearchClassifiesExternalExecutionFailure$' -count=1` aborted auto search with a generic `exit status 2`, losing both safe fallback and diagnostic stderr.
- GREEN: the same focused command passed after auto began recording `execution_failed` attempts and continuing to builtin, while explicit-engine failure preserves engine name and stderr in the error.
- Regression: `go test ./internal/app`, `go test ./...`, and `git diff --check` passed.
- Contract changes: `engine_failures` distinguishes safe auto fallback from the engine that produced the final result; explicit failures cannot become false zero results.

## Slice 3 — New files make ready metadata stale

- Test: `TestMapMarksReadyMetadataStaleAfterNewFile` indexes a temporary repository, adds one direct file, and invokes map through `app.Run`.
- RED: `go test ./internal/app -run '^TestMapMarksReadyMetadataStaleAfterNewFile$' -count=1` returned only `state: "ready"` with the old one-file projection and no freshness evidence.
- GREEN: the same focused command passed after map nodes gained additive freshness, partial, and issue fields plus direct-file comparison against metadata.
- Regression: `go test ./internal/app` and `go test ./...` passed.
- Contract changes: `state` remains compatible metadata presence; checked direct-file drift is reported separately as `freshness: "stale"` with a `new_file` issue.

## Slice 4 — Content changes cannot hide behind unchanged stat data

- Test: `TestMapMarksReadyMetadataStaleAfterSameStatContentChange` replaces file contents with the same byte length and restores the original modtime.
- RED: `go test ./internal/app -run '^TestMapMarksReadyMetadataStaleAfterSameStatContentChange$' -count=1` returned `freshness: "fresh"` because evaluation hashed only after size or second-resolution modtime changed.
- GREEN: the same focused command passed after freshness evaluation always compared the current SHA-256 fingerprint with metadata.
- Regression: `go test ./internal/app` and `go test ./...` passed.
- Contract changes: checked freshness is content-based; old summaries cannot remain under a checked-fresh node after same-stat modification.

## Slice 5 — Rename drift removes stale file projections

- Test: `TestMapReportsRenameAndRemovesDeletedFileProjection` indexes `old.txt`, renames it to `new.txt`, and inspects the public map payload.
- RED: `go test ./internal/app -run '^TestMapReportsRenameAndRemovesDeletedFileProjection$' -count=1` exposed new/deleted drift but still listed `old.txt` as though it were current.
- GREEN: the same focused command passed after deleted metadata projections were filtered from current map files while issues retained the historical path.
- Regression: `go test ./internal/app` and `go test ./...` passed.
- Contract changes: rename is represented as deterministic new/deleted drift and stale file projections are not presented as current content.

## Slice 6 — Ignore policy drift is explicit

- Test: `TestMapReportsIgnorePolicyDriftAndRecomputesVisibleFiles` changes `.aaignore` so one previously indexed file leaves the visible scope.
- RED: `go test ./internal/app -run '^TestMapReportsIgnorePolicyDriftAndRecomputesVisibleFiles$' -count=1` recomputed stale/deleted drift but exposed the policy file only as an ordinary `modified` issue.
- GREEN: the same focused command passed after added, modified, or removed `.gitignore`/`.aaignore` files were classified as `ignore_policy_changed`.
- Regression: `go test ./internal/app` and `go test ./...` passed.
- Contract changes: visible scope is recomputed with current blacklist rules and policy drift has a dedicated issue kind.

## Slice 7 — Unindexed parents do not hide indexed descendants

- Test: `TestMapExposesIndexedDescendantBelowUnindexedParent` indexes only a deep directory, then maps its unindexed ancestor.
- RED: `go test ./internal/app -run '^TestMapExposesIndexedDescendantBelowUnindexedParent$' -count=1` returned the root unindexed node with no children, partial marker, issue, or continuation.
- GREEN: the same focused command passed after unindexed nodes continued structural child traversal while marking metadata-backed details uninspected and providing `next`.
- Regression: `go test ./internal/app` and `go test ./...` passed.
- Contract changes: unindexed means metadata missing, not descendant absence; structural child stubs remain navigable.

## Slice 8 — Corrupt metadata is isolated to its node

- Test: `TestMapKeepsHealthySiblingWhenChildMetadataIsCorrupt` corrupts one child metadata file beside a healthy indexed sibling.
- RED: `go test ./internal/app -run '^TestMapKeepsHealthySiblingWhenChildMetadataIsCorrupt$' -count=1` aborted the entire map with `SERIALIZATION_ERROR`.
- GREEN: the same focused command passed after corrupt metadata became a `corrupt` partial node with a structured issue and parent partial propagation.
- Regression: `go test ./internal/app` and `go test ./...` passed.
- Contract changes: corrupt nodes preserve structural traversal and unaffected sibling routing.

## Slice 9 — Unreadable metadata is isolated to its node

- Test: `TestMapKeepsHealthySiblingWhenChildMetadataIsUnreadable` removes read permission from one child metadata file beside a healthy sibling.
- RED: `go test ./internal/app -run '^TestMapKeepsHealthySiblingWhenChildMetadataIsUnreadable$' -count=1` aborted the entire map with `IO_ERROR`.
- GREEN: the same focused command passed after metadata, ignore-policy, and directory read failures became `unreadable` partial nodes with structured issue kinds.
- Regression: `go test ./internal/app` and `go test ./...` passed.
- Contract changes: unreadable is distinct from corrupt and missing; unaffected routing remains usable.

## Slice 10 — Directory symlinks use one follow policy

- Test: `TestIndexEvalAndMapFollowInRootDirectorySymlink` creates an in-root skill alias and exercises index, index eval, and map through public commands.
- RED: `go test ./internal/app -run '^TestIndexEvalAndMapFollowInRootDirectorySymlink$' -count=1` showed index following the alias while eval treated it as a changed file and map omitted it.
- GREEN: the same focused command passed after eval and map used the same symlink-aware directory inspection as index.
- Regression: `go test ./internal/app` and `go test ./...` passed.
- Contract changes: in-root directory symlinks are followed by index, eval, and map; safety boundaries are specified by slice 11.

## Slice 11 — Symlink boundaries prevent cycles and scope escape

- Tests: `TestMapStopsAtOutOfRootSymlinkBoundary` and `TestMapStopsAtDirectorySymlinkCycle` exercise out-of-root and ancestor-cycle targets; the cycle fixture also invokes index and eval.
- RED: the out-of-root focused test showed map following the target as an ordinary unindexed subtree with no boundary issue. The prior recursion path had no realpath ancestor guard.
- GREEN: focused boundary tests passed after adding resolved-root containment and per-branch ancestor identity checks.
- Regression: `go test ./...` and `git diff --check` passed after index and eval adopted the same boundary policy.
- Contract changes: in-root non-cyclic directory symlinks are followed; out-of-root, unreadable, and cyclic targets are stopped and classified explicitly.

## Slice 12 — Ignore semantics are disclosed accurately

- Test: `TestMapDeclaresBlacklistOnlyIgnorePolicy` uses an `.aaignore` negation entry and inspects the public map contract.
- RED: `go test ./internal/app -run '^TestMapDeclaresBlacklistOnlyIgnorePolicy$' -count=1` applied blacklist behavior but emitted no indication that re-inclusion was unsupported.
- GREEN: the same focused command passed after map JSON and text added ignore-policy mode, sources, and negation support metadata.
- Regression: `go test ./...` and `git diff --check` passed.
- Contract changes: `.gitignore` and `.aaignore` are explicitly a blacklist subset in this release; `!` entries are not claimed as supported.

## Slice 13 — Default map output is semantically bounded

- Test: `TestMapDefaultsBoundWideAndDeepTreesWithContinuations` specifies bounded direct files and depth with deterministic continuation commands.
- RED: `go test ./internal/app -run '^TestMapDefaultsBoundWideAndDeepTreesWithContinuations$' -count=1` returned all 25 files and the complete deep hierarchy with no omission metadata.
- GREEN: the same focused command passed after map gained bounded options, default depth 2, default file cap 20, and structured omissions.
- Regression: legacy full-traversal tests were made explicit with `--full`; `go test ./internal/app` and `go test ./...` passed.
- Contract changes: bounded is the default projection; every omitted file group or directory branch has an exact continuation command.

## Slice 14 — Full projection is explicit and complete

- Test: `TestMapFullRecoversCompleteHierarchy` requests `--full` for a 25-file, four-level indexed fixture.
- RED: `go test ./internal/app -run '^TestMapFullRecoversCompleteHierarchy$' -count=1` recovered all content but omitted any machine-readable indication that bounds were disabled.
- GREEN: the same focused command passed after map added projection metadata with full/bounded mode and effective controls.
- Regression: `go test ./...` passed.
- Contract changes: `--full` disables semantic depth/file bounds subject only to reported partial errors, and the result declares that decision.

## Slice 15 — Oversized text maps use managed continuation

- Test: `TestMapOversizedTextSpillsToManagedOutput` requests a full map large enough to exceed the shared inline-output limit.
- RED: `go test ./internal/app -run '^TestMapOversizedTextSpillsToManagedOutput$' -count=1` emitted the complete unbounded map and no output identifier or continuation command.
- GREEN: the same focused command passed after map results opted into the shared large-output transport with `text` as their primary field.
- Contract changes: text maps remain bounded independently of semantic map bounds and disclose the managed output ID, emitted range, total runes, and exact next slice command.

## Slice 16 — Oversized JSON maps do not duplicate their payload

- Test: `TestMapOversizedJSONStoresCompletePayloadWithoutInlineDuplication` verifies the envelope is bounded and reconstructs the stored JSON through the shared managed-output API.
- RED: `go test ./internal/app -run '^TestMapOversizedJSONStoresCompletePayloadWithoutInlineDuplication$' -count=1` returned roughly 34K runes because the full map remained under `data.value` beside transport metadata.
- GREEN: focused text and JSON transport tests passed after JSON delivery began storing the serialized map and replacing non-map inline data with only the preview field plus transport.
- Regression: `go test ./internal/output ./internal/command` passed.
- Contract changes: a truncated JSON map has `data.text` plus `data.transport`; its sole complete payload is the JSON artifact referenced by `output_id`.

## Slice 34 — Skill, schema, source help, and bundled binary agree

- Test: `TestBundledSkillBinaryMatchesRetrievalContract` executes bundled `map --help` and `search --help`, compares them byte-for-byte with source help, and checks the additive map schema fields.
- RED: `go test ./internal/app -run '^TestBundledSkillBinaryMatchesRetrievalContract$' -count=1` failed because `skills/indexing-folder/bin/aascribe` was absent.
- Version check: after source was versioned as `0.2.0`, the focused test failed against the first rebuilt `0.1.0` bundle and then passed after the final rebuild.
- GREEN: the focused command passed after rebuilding the bundled binary from the current versioned source with `go build -trimpath -ldflags='-s -w'`.
- Contract changes: installing the repository skill now supplies a `0.2.0` executable whose retrieval help and version match the checked-in CLI contract rather than relying on an unrelated `PATH` version.

## Final regression and contract audit

- Representative public fixtures cover fresh, stale, unindexed, ignored, corrupt, unreadable, symlinked, wide, deep, bounded, full, text-spilled, and JSON-spilled maps plus the shared search-engine contract cases.
- `go test ./...` passed.
- `go vet ./...` passed.
- `git diff --check` passed.
- `jq empty docs/shapes/PathIndexMap.schema.json docs/shapes/SearchResult.schema.json` passed.
- The source-scoped stale-claim audit returned no matches across the active skill, usage, migration, schema, and CLI help sources.
- `skills/indexing-folder/bin/aascribe --version` reports `0.2.0`.
