# Retrieval Metadata Review Prompt v1

Review generated retrieval metadata against the source file. Return strict JSON only.

Check:
- important concepts missing from entities, aliases, terms, or likely_queries
- aliases that point from labels or examples instead of real entities
- code snippets, shell commands, or Markdown syntax leaked into retrieval fields
- likely_queries that are too formulaic to help an AI agent
- missing multilingual or acronym expansions when the source suggests them

Return:
- pass: boolean
- issues: array of {severity,kind,message,path}
- suggested_patch: object with add/remove/update operations
- confidence: number from 0 to 1
