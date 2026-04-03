# Provider Evaluation

## Purpose

`mindp` already has a provider seam because not every workload should run on the same browser backend.

The provider question is not only “what works.” It is also:

- what can be maintained
- what can be debugged
- what fits the dependency and ownership model
- what level of hostile-target support is actually required

## Provider Categories

### Native local Chromium

Best fit for:

- small owned deployments
- internal tools
- authenticated scraping on moderate targets
- minimal dependency and operational surface

Strengths:

- simplest deployment model
- strongest alignment with `mindp`’s small-core design
- direct control over launch, persona, behavior, and CDP usage

Limits:

- cannot solve browser-internal stealth gaps by itself

### Remote CDP provider

Best fit for:

- connecting to externally managed Chromium
- separating orchestration from browser hosting
- evaluating alternative browser runtimes without rewriting the control plane

Strengths:

- keeps `mindp` as the same automation interface
- allows experimentation with hosted or hardened backends

Limits:

- the browser-side trust model and patch quality live outside `mindp`

### External hardened browser providers

Examples in this category include:

- patched Chromium environments
- Patchright-style runtimes
- Camoufox-style hardened browser stacks
- future CDP-compatible experimental runtimes

Best fit for:

- hostile anti-bot environments
- workloads that have already outgrown native Chromium stealth

Strengths:

- access to browser-internal mitigation that a normal CDP client cannot provide

Limits:

- larger operational footprint
- higher maintenance and debugging cost
- more moving parts outside the Go codebase

## Recommendation

Use this decision order:

1. start with native `mindp` on local Chromium
2. move to remote CDP when browser hosting or orchestration needs separate concerns
3. adopt an external hardened provider only when empirical target behavior justifies the extra cost

That keeps the default path simple while preserving a realistic upgrade path for harder targets.
