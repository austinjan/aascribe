# Retrieval Prompt Patch Prompt v1

Given a retrieval review report and the current retrieval prompt, propose a safer prompt revision.

Return strict JSON only:
- target_prompt_path
- proposed_content
- rationale
- expected_improvements
- rollback_note

Do not modify metadata directly. The caller will checkpoint, patch, re-index, review again, then accept or restore.
