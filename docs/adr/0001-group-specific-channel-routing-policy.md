# ADR 0001: Use Core Routing Policy for Group-Specific Channel Overrides

## Status

Accepted

## Context

The Base project currently routes a model through channel-level priority and weight. The Overlay project adds optional priority and weight overrides for individual user groups. The requested migration must preserve Base's user-group semantics and must not introduce Overlay's separate pricing-group or recharge-group domain.

The new Base plugin system is designed around vendor task routes and protocol adapters. It does not currently provide a safe, deterministic hook for changing the channel candidate set before routing. Channel selection is also shared by normal relay requests, retries, concurrency handling, and model availability probing.

## Decision

Implement group-specific channel priority and weight as a core routing policy capability, not as a plugin.

Each channel keeps its existing global priority and weight. An optional per-user-group override may replace either field independently. The effective policy for a real request is resolved from the request's already-selected user group:

- configured override field: use the override;
- missing override field: inherit the global field;
- no matching group entry: use both global fields.

The `auto` group continues to resolve through the existing Base group order first. The resolved group then supplies the effective channel policy; group-specific channel overrides do not change the `auto` group order.

Active model probing remains model-level and uses the channel's global priority, then global weight, then a stable channel identifier for probe order. Group-specific overrides do not create group-specific availability states and do not affect the probe result.

The frontend will provide a Base-native visual editor and list indicators. Raw JSON is an internal persistence format, not the required administrator experience.

## Consequences

Positive consequences:

- Every relay protocol and retry path shares one routing rule.
- Existing channels and requests remain compatible without overrides.
- The plugin runtime remains outside the latency-sensitive routing path.
- Probe ordering remains deterministic even when every user group has an override.
- Base's newer channel-cache and request-policy implementation remains authoritative.

Negative consequences:

- The capability touches core routing, persistence, and channel administration code.
- The initial release does not support group-specific availability states or batch editing of overrides.
- The global priority and weight fields retain a dual role as fallback values and probe-order hints, which must be explained in the UI.

## Alternatives considered

### Implement as a task plugin

Rejected. Existing plugins run at vendor route/protocol seams, after channel selection. Adding a routing hook would require new plugin permissions, candidate serialization, failure fallback, hot-path execution, and multi-node determinism guarantees.

### Probe each user group independently

Rejected for this scope. It would change model availability from a model-level state to a model-plus-group state and require changes to the Available Models projection, blocking logic, and persistence.

### Replace global priority and weight with per-group values

Rejected. It would break old channels, make unconfigured groups ambiguous, and remove the stable ordering needed by active probing. Overrides must remain additive.
