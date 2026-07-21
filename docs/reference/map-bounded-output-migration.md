# Map bounded-output migration

`aascribe map <path>` now returns a semantically bounded projection by default: directory depth `2` and at most `20` direct files per directory. This keeps routing output predictable before the shared managed-output transport becomes necessary.

Consumers that previously relied on the complete unbounded hierarchy must request it explicitly:

```bash
aascribe map <path> --full
```

Machine consumers should inspect `projection.mode` and follow each node's structured `omitted[].next` command. A bounded map is navigable but is not evidence that omitted branches are absent. `--summary-only` suppresses direct-file details, while `--depth` and `--max-files-per-dir` tune the bounded projection.

Oversized text and JSON can still spill after projection. In that case the inline response contains a preview and `transport`; retrieve the sole complete payload through its `output_id`.

## SearchResult compatibility

`SearchResult.match_count` remains available, but it now explicitly means returned matching lines and is a compatibility alias for `returned_match_lines`. It is not an occurrence count.

New consumers should also read:

- `query_mode` and `match_unit` before interpreting matches
- `column_unit` before locating a match in Unicode text
- `scope` before deciding whether the searched area was adequate
- `complete`, `partial_reasons`, `exclusions`, and `engine_failures` before making an absence claim

Fixed-string engines share the declared candidate-file projection. Regex auto mode uses compatible `rg` and builtin implementations only; explicit `grep` and `git-grep` regex requests are rejected rather than silently changing query semantics.
