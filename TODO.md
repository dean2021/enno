# Enno SDK Improvement Plan

This plan focuses on turning Enno from a useful v0 agent runtime into a cleaner,
more stable Go SDK. The priority is to improve SDK ergonomics without breaking
the existing `Agent.Run(ctx, input) (string, error)` path.

## Goals

- Keep the root `enno` package provider-neutral and small.
- Preserve source compatibility for current SDK and CLI users where practical.
- Make run results, state, tool execution, and provider options explicit.
- Move product-specific behavior out of the core agent loop.
- Improve observability and control without forcing users into the CLI model.

## Non-Goals

- Do not redesign the CLI UI as part of the SDK cleanup.
- Do not remove OpenAI-compatible or Anthropic providers.
- Do not require users to adopt streaming or advanced hooks for basic usage.
- Do not introduce a large dependency-heavy schema framework unless needed.

## Phase 1: Result API Without Breaking `Run` [done]

### Problem

`Agent.Run(ctx, input)` returns only final text. SDK users cannot reliably inspect
usage, stop reason, round count, tool calls, errors from individual tools, or the
messages produced during a run.

### Tasks

- [x] Add `RunDetailed(ctx context.Context, input string) (RunResult, error)`.
- [x] Keep `Run(ctx, input)` as a compatibility wrapper around `RunDetailed`.
- [x] Add `RunResult` with:
  - [x] `Content string`
  - [x] `Messages []Message`
  - [x] `Usage Usage`
  - [x] `Rounds []RoundResult`
  - [x] `StopReason StopReason`
  - [x] `Duration time.Duration`
- [x] Add `RoundResult` with:
  - [x] `Round int`
  - [x] `ModelUsage Usage`
  - [x] `Assistant Message`
  - [x] `ToolCalls []ToolCallResult`
  - [x] `Duration time.Duration`
- [x] Add `ToolCallResult` with:
  - [x] `Call ToolCall`
  - [x] `Result string`
  - [x] `Err error`
  - [x] `Duration time.Duration`
- [x] Add `StopReason` values:
  - [x] `StopReasonEndTurn`
  - [x] `StopReasonMaxToolRounds`
  - [x] `StopReasonError`
  - [x] `StopReasonCanceled`
- [x] Update tests to verify `Run` remains compatible.
- [x] Update README package examples to show `RunDetailed` for advanced users.

### Acceptance Criteria

- [x] Existing public `Run` behavior is unchanged.
- [x] CLI can continue using `Run` or migrate incrementally to `RunDetailed`.
- [x] Unit tests cover normal text response, tool call response, max rounds, and provider error.

## Phase 2: Explicit Session State [done]

### Problem

`Agent` is currently both a runner and a mutable conversation store. This is simple,
but less ergonomic for services that need to load, persist, fork, or run multiple
sessions.

### Tasks

- [x] Add `Session` type:
  - [x] `Messages []Message`
  - [x] `Append(Message)`
  - [x] `Clone() Session`
  - [x] `Reset()`
- [x] Add `Agent.RunSession(ctx, session *Session, input string) (RunResult, error)`.
- [x] Keep current `Agent.Messages()` and `Agent.Reset()` behavior for compatibility.
- [x] Internally migrate `Agent.history` to use `Session` or a session-like helper.
- [x] Add examples for:
  - [x] stateless request handling
  - [x] loading/saving session JSON
  - [x] forking a session for speculative runs

### Acceptance Criteria

- [x] Existing `Agent` users do not need to change code.
- [x] Service-style users can run a request without relying on hidden agent history.
- [x] `Session.Clone()` performs a safe deep enough copy for messages and tool calls.

## Phase 3: Tool Result Structure [done]

### Problem

Tool handlers return only `(string, error)`. This mixes model-visible content,
execution metadata, and tool failure state into one string.

### Tasks

- [x] Add `ToolResult`:
  - [x] `Content string`
  - [x] `Error bool`
  - [x] `Metadata map[string]any`
- [x] Add `StructuredToolHandler func(context.Context, json.RawMessage) (ToolResult, error)`.
- [x] Add `NewStructuredTool(...) Tool`.
- [x] Keep `NewTool` and `NewTypedTool` as compatibility helpers.
- [x] Extend events and `RunResult` to preserve structured tool result metadata.
- [x] Decide how `ToolResult.Error` maps to provider tool-result messages.
- [x] Update built-in tools gradually to return structured results internally.

### Acceptance Criteria

- [x] Existing tools keep working unchanged.
- [x] New structured tools can attach metadata without exposing it to the model.
- [x] CLI can render tool metadata separately from model-visible content.

## Phase 4: Typed Tool Schema Builder [done]

### Problem

`Tool.Properties map[string]any` is flexible but easy to get wrong. It also makes
tool definitions noisy for common typed Go structs.

### Tasks

- [x] Add a small schema builder API:
  - [x] `SchemaObject()`
  - [x] `StringProp(name)`
  - [x] `IntegerProp(name)`
  - [x] `BooleanProp(name)`
  - [x] `EnumProp(name, values...)`
  - [x] `Required(names...)`
- [x] Add optional `NewTypedToolFromSchema[T]`.
- [x] Consider `jsonschema` generation from structs only if the builder is insufficient.
- [x] Validate tool names and required fields at `NewAgent` time.
- [x] Return clear errors for duplicate tool names.

### Acceptance Criteria

- [x] Existing map-based schemas still work.
- [x] Common tool declarations become shorter and less error-prone.
- [x] Duplicate or invalid tool definitions fail early.

## Phase 5: Provider Request Options [done]

### Problem

`Provider.Complete(ctx, Request)` has no standardized place for call-level options
such as temperature, max output tokens, tool choice, response format, or metadata.

### Tasks

- [x] Add `RequestOptions` to `Request`:
  - [x] `Temperature *float64`
  - [x] `MaxOutputTokens int64`
  - [x] `ToolChoice ToolChoice`
  - [x] `ResponseFormat ResponseFormat`
  - [x] `Metadata map[string]string`
- [x] Add corresponding fields to `Config` for default options.
- [x] Define provider behavior for unsupported options:
  - [x] ignore silently for purely optional hints, or
  - [x] return `ErrUnsupportedOption` for strict options.
- [x] Update OpenAI-compatible and Anthropic providers to map supported options.
- [x] Add provider tests for option translation.

### Acceptance Criteria

- [x] Users can set common generation parameters without provider-specific code.
- [x] Providers remain free to expose provider-specific config in their own packages.
- [x] Unsupported behavior is documented and tested.

## Phase 6: Move Core Policies Out of Agent Loop [done]

### Problem

The core `Agent` loop currently knows about task graph tool names and compaction
tool behavior. This makes the root package less generic and harder to reason about.

### Tasks

- [x] Introduce a lightweight policy interface, for example:
  - [x] `BeforeModel(ctx, *RunState) error`
  - [x] `AfterModel(ctx, *RunState, Response) error`
  - [x] `AfterTools(ctx, *RunState, []ToolCallResult) error`
- [x] Move task graph reminder logic into a `TaskReminderPolicy`.
- [x] Move compaction into a `ContextManager` or `CompactionPolicy`.
- [x] Keep old `Config.Compaction` as a compatibility shim that installs the policy.
- [x] Keep task graph behavior enabled only when the relevant tool package/config chooses it.

### Acceptance Criteria

- [x] Root loop no longer hard-codes task tool names.
- [x] Compaction behavior remains compatible for existing configs.
- [x] Policies can be tested independently from provider adapters.

## Phase 7: Middleware and Control Hooks [done]

### Problem

`EventHandler` is useful for observation but cannot approve tools, block actions,
alter requests, inject tracing IDs, or customize retries.

### Tasks

- [x] Add hook points that can control execution:
  - [x] `BeforeProviderCall`
  - [x] `AfterProviderCall`
  - [x] `BeforeToolCall`
  - [x] `AfterToolCall`
- [x] Define hook return behavior:
  - [x] continue
  - [x] abort
  - [x] replace request/result
  - [x] mark tool denied
- [x] Add examples:
  - [x] require user approval before shell tool
  - [x] redact secrets before provider call
  - [x] collect OpenTelemetry spans
- [x] Keep `EventHandler` as a simple observation API.

### Acceptance Criteria

- [x] Users can implement tool approval without forking built-in tools.
- [x] Observability and control are separate concepts.
- [x] Hooks are optional and do not complicate basic usage.

## Phase 8: Streaming Support [done]

### Problem

The provider interface only supports complete responses. CLI and apps that want
incremental output need a first-class streaming contract.

### Tasks

- [x] Add optional provider interface:
  - [x] `Stream(ctx, Request) (Stream, error)`
- [x] Add stream event types:
  - [x] text delta
  - [x] thinking delta if provider exposes it
  - [x] tool call delta
  - [x] final response
  - [x] usage
- [x] Add `Agent.RunStream(...)` or stream-aware `RunDetailed` variant.
- [x] Update OpenAI-compatible and Anthropic providers where SDK support is stable.
- [x] Keep non-streaming providers valid.

### Acceptance Criteria

- [x] Existing providers do not need to implement streaming.
- [x] Streaming can produce the same final `RunResult` as non-streaming.
- [x] CLI can consume streaming without parsing provider-specific chunks.

## Phase 9: Built-In Tool Consistency [done]

### Problem

Built-in tools use slightly different conventions for limits, names, truncation,
timeouts, and errors.

### Tasks

- [x] Define shared conventions:
  - [x] timeout defaulting
  - [x] max output chars
  - [x] truncation suffix
  - [x] error formatting
  - [x] root/workdir validation
- [x] Add common helper package under `tools/internal` if needed.
- [x] Add config fields for output limits in filesystem, shell, grep, glob, and subagent.
- [x] Make shell safety policy explicit and configurable.
- [x] Audit built-in tools for path escape tests and output truncation tests.

### Acceptance Criteria

- [x] Built-in tools feel consistent to SDK users.
- [x] Tool output limits are configurable.
- [x] Safety behavior is documented and covered by tests.

## Phase 10: Documentation and Stability [done]

### Problem

The project has useful docs, but the SDK stability story is not yet explicit.

### Tasks

- [x] Add an SDK guide separate from CLI usage.
- [x] Document public API compatibility guarantees for `v0.x`.
- [x] Add examples for:
  - [x] minimal agent
  - [x] custom provider
  - [x] custom typed tool
  - [x] persistent session
  - [x] structured tool results
  - [x] hooks/middleware
- [x] Add a migration guide for each breaking change.
- [x] Add package-level Go docs for root package and provider packages.

### Acceptance Criteria

- [x] New SDK users can start without reading CLI docs.
- [x] Advanced users can understand extension points from docs alone.
- [x] Breaking changes are intentional and documented.

## Suggested Execution Order

1. Implement `RunDetailed` and `RunResult`.
2. Add `Session` without changing existing `Agent.Run`.
3. Add structured tool results behind new constructors.
4. Add provider request options.
5. Extract compaction and task reminder into policies.
6. Add middleware hooks.
7. Add streaming.
8. Normalize built-in tools.
9. Update docs and examples after each phase.

## Compatibility Rules

- Keep old constructors and methods until a planned breaking release.
- Prefer additive APIs during the `v0.x` series.
- When breaking changes are necessary, provide a migration path in docs.
- Avoid moving packages unless the old path can remain as a shim.
- Do not make CLI-specific behavior mandatory in the SDK.

## Open Questions

- Should `RunDetailed` return partial results when the run ends with an error?
- Should tool execution errors be model-visible by default or represented as structured failures?
- Should compaction belong to root `enno`, `tools/compact`, or a new `memory` package?
- Should provider-specific options be embedded in `RequestOptions`, or only in provider configs?
- How much JSON schema generation should the SDK own versus delegating to external libraries?
