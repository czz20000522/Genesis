# Kernel Design Documents

Design documents answer how an approved requirement is shaped.

A design document covers:

- boundary and owner;
- data flow;
- protocol and schema shape;
- failure semantics;
- permission and authority model;
- recovery model;
- observability and projections;
- reference alignment or intentional differences.

Design documents must not become issue ledgers. They can reject alternatives, but they must settle on one owner-owned path before implementation begins.

Use this template:

```markdown
# Design: <Capability Name>

## Requirement

Link the approved requirement.

## Boundary And Owner

State the owner and the surfaces that are explicitly outside the owner.

## Data Flow

Show how input, kernel facts, provider context, tool effects, and projections move.

## Protocol

Define request, response, event, projection, and error shapes at the level needed
for implementation.

## Failure Semantics

Separate invalid requests, policy blocks, executed failures, infrastructure
failures, provider failures, and recovery behavior.

## Permission And Authority

State who can request what, which fields are kernel-owned, and how policy is
resolved.

## Recovery And Observability

State what is durable, how replay works, and which projections expose which
facts.

## Rejected Alternatives

Record paths that were considered and rejected.
```
