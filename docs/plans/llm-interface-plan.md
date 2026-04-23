# LLM Interface Plan For `aascribe`

## Summary

Implement a first LLM subsystem before `describe` or `index`, with a narrow MVP scope:

- Gemini is the only supported provider
- API credentials are loaded from a checked-in user-managed config file at `<store>/config.toml`
- the LLM layer exposes a stable internal interface that later commands can reuse
- `describe` and `index` will depend on this interface rather than talking to Gemini directly

The goal of this phase is not to implement all AI features. It is to create one reliable integration seam for request building, config loading, HTTP transport, response parsing, and summary-oriented error handling.

## Implementation Changes

- Add a dedicated internal LLM module tree, separate from command dispatch:
  - config loading
  - Gemini client
  - request/response types
  - provider-agnostic trait/interface
  - summarization entrypoints
- Keep the interface provider-shaped even though MVP supports only Gemini, so later providers can be added without rewriting callers.
- Define one primary internal interface:
  - `LlmClient` trait with a summary-oriented generation method
  - first concrete implementation: `GeminiClient`
- Resolve config from the existing store path:
  - config file path: `<store>/config.toml`
  - this keeps runtime configuration co-located with the store already created by `aascribe init`
- Add a new config section for the LLM layer:
  - provider name
  - Gemini API key
  - model name
  - timeout settings
  - optional max token / temperature style tuning fields only if Gemini support requires them for MVP
- Make Gemini the only valid provider in MVP:
  - if config names another provider, return a structured configuration error
  - do not add provider selection CLI flags yet
- Add a summary-focused service on top of the raw client:
  - input: source text plus summary options
  - output: normalized summary result usable by `describe` and later by `index`
- Normalize Gemini responses into internal structs rather than leaking provider-specific JSON into callers.
- Fail cleanly when config or credentials are missing:
  - no panic
  - no silent fallback to environment variables in MVP
  - return a structured runtime error that explains the expected config location and keys

## Public Interfaces And Config Contract

- No new user-facing CLI command is required in this phase.
- The primary new public-adjacent contract is the config file at `<store>/config.toml`.

Recommended MVP config shape:

```toml
[llm]
provider = "gemini"
model = "gemini-2.5-flash"
api_key = "your-gemini-api-key"
timeout_seconds = 30
```

- `provider` is required and must equal `gemini`
- `model` is required for explicitness; do not hide model choice in code only
- `api_key` is required in the config file for MVP
- `timeout_seconds` is optional in spirit but should be written and supported as a first-class setting

Internal interface shape to build toward:

- `LlmClient`
  - accepts a normalized generation request
  - returns a normalized generation response
- `Summarizer`
  - accepts text plus summary options such as `short`, `medium`, `long`
  - returns normalized summary content and basic metadata

Recommended internal request shape:

- model
- prompt/system instructions
- user content
- timeout

Recommended internal summary response shape:

- summary text
- provider
- model
- finish reason if available
- token usage fields only when Gemini returns them reliably

## Gemini Integration Decisions

- Use the Gemini REST API directly over HTTP for MVP.
- Do not introduce a third-party SDK unless the Go ecosystem forces it for correctness; prefer a simple HTTP client plus typed JSON structs.
- Keep prompt construction inside the summarization layer, not inside command handlers.
- Start with text-only summarization support.
- Do not support streaming in MVP.
- Do not support tool calling, multimodal input, or structured JSON generation from Gemini in this phase unless needed to make `describe` work.
- Use deterministic settings where possible so repeated summaries are reasonably stable for tests and user expectations.

## Error Handling And Behavior

- Missing `<store>/config.toml` should return a clear config-not-found error.
- Missing `[llm]` section or missing required keys should return a clear invalid-config error.
- Invalid Gemini credentials should return a provider-auth error.
- Network timeouts and transport failures should return retryable runtime errors.
- Provider response parsing failures should return provider-response errors and preserve as much raw context as is safe for debugging.
- The LLM layer should not print directly to stdout/stderr; it returns typed errors to the command layer.

## Test Plan

- Config loading tests:
  - missing config file
  - malformed TOML
  - missing `[llm]`
  - unsupported provider
  - valid Gemini config
- Client construction tests:
  - valid config creates a Gemini client
  - unsupported provider is rejected
- Request/response tests:
  - Gemini request serialization matches the expected API shape
  - Gemini response parsing handles normal success payloads
  - Gemini response parsing handles empty or malformed candidate payloads
- Summarizer tests:
  - short/medium/long modes map to different prompt variants or summary options
  - summarizer returns normalized output independent of Gemini wire format
- Integration-style tests with mocked HTTP:
  - successful summary call
  - auth failure
  - timeout
  - server error

## Follow-On Work Enabled By This Plan

- Implement `describe` as the first real consumer of the LLM subsystem
- Add file extraction and truncation rules ahead of summarization
- Reuse the same summarizer from `index` for changed-file summaries
- Add fallback summarization or alternate providers later without changing command handlers

## Assumptions

- MVP supports Gemini only
- API keys are stored in `<store>/config.toml`, not environment variables
- The existing store path resolution logic remains the single way to locate config
- The first model default in docs/examples is `gemini-2.5-flash`
- This phase focuses on internal interface and config plumbing, not on implementing `describe` or `index` yet
