---
name: llmagent
description: Delegate prompts to other LLM backends. Before first use, run `llmagent models` to see available models and their descriptions, then choose the best model for each task based on the description.
---

# llmagent

Delegate prompts to alternative LLM backends.

## First-time setup

Run `llmagent models` to see all configured models and their descriptions.
Use the description to decide which model fits each task.

## Usage

```bash
llmagent models                                    # List available models (do this first)
llmagent -m <model-id> "prompt"                    # Run prompt on a model
llmagent -m <model-id> --summary "prompt"          # Run + summarize via AI after completion
llmagent -m <model-id> --summary --summary-prompt "Focus on X" "prompt"
llmagent -m <model-id> --async "prompt"            # Run async, returns session ID
llmagent daemon status                             # Check sessions
llmagent daemon attach <session-id>                # Attach to a session
llmagent test-summary                              # Test summary API configuration
```

## When to use

- The user explicitly invokes this skill or names a model from the list
- You need to delegate a self-contained, independent sub-task to a different model
- Your plan identifies modules that are uncoupled and can be implemented in parallel

## When NOT to use

- During the planning phase — plan first, delegate later
- For tasks that depend on the output of another module being implemented first

## Summary (recommended)

Use `--summary` whenever possible. Instead of streaming raw agent output (which may be verbose or include intermediate tool calls), the output is captured and summarized by a configured AI model before being reported back. This keeps responses focused and actionable.

When writing a summary prompt via `--summary-prompt`, focus on what matters for the task:

- Did the agent accomplish the goal? What changed?
- What key decisions were made and why?
- Were there any errors, unexpected behaviors, or deviations?
- What remains to be done, if anything?
- For code tasks: what files were changed, what approach was taken, any design trade-offs noted?

The default built-in summary prompt already covers these points, so `--summary-prompt` is only needed when you want to emphasize specific aspects.

## Test before delegating

Before sending a task to a sub-agent, write a test case that validates the expected outcome. And tell the agent to use that test case as the success criteria for the task. This way, when the agent returns, you can automatically verify whether it succeeded or failed based on the test results, rather than just trusting the agent's self-reported success.

```bash
# Example: delegate a bug fix, then verify
llmagent -m <model> --summary "fix the null pointer in handler.go"
# After completion, run the test
go test ./... -run TestHandler
# If tests fail, re-delegate with the failure output
```

## Output behavior by backend

When NOT using `--summary`, agent output differs by backend:

- **opencode**: Shows full tool-call details — file reads, edits, shell commands, and their outputs. Useful for debugging or when you need to audit exactly what the agent did step by step.
- **claude-code**: Shows only the final response output. Tool calls and intermediate steps are hidden, so you only see the conclusion.
- **codex**: Shows tool-call details similar to opencode.

Prefer `--summary` to normalize output across backends and get a consistent, concise report.

## Strategy

1. **Plan first**: Break the task into independent, uncoupled modules. Design clear interfaces between them (API contracts, data shapes, function signatures).
2. **Design in detail**: Before delegation, write the concrete file ownership, public interfaces, data flow, edge cases, acceptance tests, and integration points. A vague module name is not enough.
3. **Keep delegation small**: Do not ask a sub-agent to build a large feature, broad module, or open-ended subsystem. Split work into bounded tasks that can be reviewed quickly and verified independently.
4. **Write tests**: For each module, define the expected behavior as a test case before delegating.
5. **Delegate in parallel**: Dispatch independent, bounded tasks to different models simultaneously via `--async`. Tasks that don't share state or dependencies can run at the same time.
6. **Implement critical glue yourself**: Keep architecture, cross-module contracts, integration, and risky fixes in the main agent's hands when delegation would create ambiguity.
7. **Verify**: When agents return, validate results against test cases. Only proceed to integration when all modules pass individually.
8. **Example — web project**:
   - Design the API spec first (shared interface)
   - Write a contract test for the API (status codes, response shapes)
   - Dispatch small backend and frontend slices to different models in parallel
   - Verify backend passes API contract tests, frontend passes UI tests
   - Components with no external deps (e.g. a standalone UI widget) can be sub-delegated

## Rules

- Once the user explicitly invokes this skill, use `llmagent` for delegation — do NOT use the agent's built-in subagent/spawn capabilities
- Prefer `--summary` for cleaner, focused results
- Prefer calling multiple agents in parallel when modules are uncoupled
- Each agent invocation is isolated — no shared state, no implicit coupling
- Do not delegate especially large modules. If a task cannot be described with clear file boundaries, expected behavior, and tests, refine the design or implement the uncertain part yourself first
- Keep hands-on implementation balanced: the main agent should write or integrate enough code to own the final design, but avoid personally writing more than roughly 60% of the total implementation when delegation is the chosen strategy
