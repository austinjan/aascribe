# Product Policy

## Model Support

`aascribe` supports the newest practical LLM model available for its active provider.
The product does not optimize for old model compatibility, legacy prompt behavior, or degraded-model parity.

Current policy:

- support one current model line as the default product path
- allow explicit model override only for short-term testing, migration, or debugging
- treat older model behavior as unsupported unless there is a concrete product reason to keep it
- remove old-model workarounds when they reduce capability, code clarity, or agent effectiveness

The product is built for AI agents, so the model is part of the core runtime surface, not a replaceable low-power accessory.

## Capability First

Capability is the first product priority.

Stronger LLMs can reason over larger context, recover from incomplete instructions, produce better plans, use tools more effectively, and identify risks that static code or weaker models miss. `aascribe` should expose that power instead of designing around the limitations of older models.

Product decisions should therefore prefer:

- richer context over minimal context
- direct tool-use workflows over manual fallback instructions
- model-readable outputs over human-only formatting
- explicit next-step guidance over passive command output
- durable artifacts that let an agent resume, inspect, and verify work

## Guardrails

Security, privacy, cost control, and accuracy remain product requirements, but they should be handled as targeted controls, not broad reductions in AI capability.

Acceptable controls:

- keep secrets out of prompts, logs, generated docs, and config files
- require explicit user confirmation for destructive or persistent external actions
- record provider, model, token usage, and report paths when available
- validate generated plans before execution
- return structured errors when credentials, config, or provider responses are invalid
- make high-risk actions inspectable and reversible where practical

Avoid controls that:

- force weaker models only because older behavior is easier to predict
- hide useful repository context from the model without a specific security reason
- replace model reasoning with brittle heuristics for complex judgment tasks
- preserve compatibility paths that make the main agent workflow worse

The policy is not "ignore safety." The policy is that safety work should improve the agent's ability to act correctly, not make the agent artificially weaker.

## Compatibility Stance

Backward compatibility is important for stored artifacts, command contracts, and user data.
It is not a goal for model-specific behavior.

Stable surfaces:

- CLI command names and documented flags
- config schema and secret-handling rules
- output shapes and machine-readable fields
- index artifacts and report locations
- explicit migration paths for durable files

Unstable surfaces:

- exact LLM wording
- prompt internals
- old model quirks
- compatibility hacks for weaker reasoning
- model-specific fallback behavior that is not part of the documented contract

When a stronger model changes behavior, evaluate whether the new behavior better serves the product goal. Do not preserve weaker output only because it is familiar.

## Decision Rule

When choosing between more capability and more restriction, default to capability unless there is a concrete, named risk.

If there is a risk, solve that risk directly.
Do not use a broad model downgrade or artificial limitation as the first answer.
