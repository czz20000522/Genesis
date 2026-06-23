Feature: Application connector runtime
  External applications connect to Genesis through connector-owned protocol boundaries.

  Scenario: Inbound external event submits one kernel turn
    Given a Feishu external event with event id "evt-1"
    And the event belongs to chat "oc_123"
    When the application connector runtime processes the event
    Then it should create a request context with an opaque kernel session id
    And it should submit one turn to the kernel
    And it should not build provider context

  Scenario: Duplicate inbound external event is suppressed before kernel side effects
    Given a Feishu external event with event id "evt-1"
    And that event has already created a kernel turn
    When the same external event is received again
    Then the connector runtime should return the existing request record
    And it should not submit another kernel turn

  Scenario: Application command creates connector outbox action
    Given a kernel typed result with semantic intent to send a message
    When application policy accepts the intent
    Then it should enqueue one connector outbox item
    And the outbox item should contain a connector idempotency key
    And it should not expose external credentials to the LLM

  Scenario: Connector delivery failure is not a kernel turn failure
    Given a connector outbox item for Feishu
    When the Feishu connector action fails with a rate limit error
    Then the connector should record a delivery receipt with retry state
    And the kernel turn facts should remain unchanged

  Scenario: External identities do not grant kernel authority
    Given an external sender has an admin role in Feishu
    When the connector maps the event into a request context
    Then the request context may record the external role as origin metadata
    And it should not set kernel permission mode, sandbox profile, approval policy, credential authority, or memory authority
