Feature: Kernel owner structure governance

  Genesis keeps owner boundaries visible in code structure, projections, and
  documents so the kernel does not grow into one central object.

  Scenario: Session projection composes owner projections
    Given the ledger contains turn, tool, job, work, and memory facts
    When a caller reads a session projection
    Then the session projection is composed from owner-owned replay helpers
    And the kernel entry point does not reimplement each owner replay rule

  Scenario: Transport delegates to owners
    Given an HTTP request targets a kernel command or inspection surface
    When the transport accepts the request
    Then it authenticates, decodes, delegates, maps errors, and encodes the response
    And it does not own ledger replay, permission policy, memory truth, work state, or tool execution semantics

  Scenario: Periodic governance retires stale documents
    Given a phase has closed and its implementation evidence has moved to the retirement log
    When architecture, feature, directory, and document review runs
    Then stale implementation plans and obsolete architecture notes are deleted or condensed
    And active requirements, designs, and issues describe only the current positive contract
