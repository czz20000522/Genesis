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

  Scenario: Kernel final text can become an ordinary connector reply
    Given a Feishu inbound external event creates a kernel turn
    And the kernel returns final text for that turn
    When application policy enables ordinary final-text delivery
    Then the connector runtime should enqueue one send-message outbox item
    And it should execute that item through the connector adapter
    And it should record a delivery receipt
    And delivery failure should not rewrite the kernel turn facts

  Scenario: Retry scheduled delivery waits until the next attempt time
    Given a connector outbox item failed with a retryable rate limit result
    And the connector recorded a next attempt time in the future
    When a delivery worker asks for eligible outbox items
    Then that item should not be eligible for execution
    And no connector adapter should be called

  Scenario: Terminal delivery state suppresses duplicate execution
    Given a connector outbox item has already reached sent state
    When a delivery worker attempts to execute it again
    Then the connector should not call the adapter
    And it should record a duplicate-suppressed delivery receipt

  Scenario: Exhausted retries move to dead letter
    Given a connector outbox item has reached the maximum retry attempts
    When the next connector delivery attempt fails with a retryable error
    Then the connector should record a dead-letter receipt
    And the item should not become eligible for automatic delivery again

  Scenario: Partial external success requires recovery instead of blind retry
    Given a connector action uploaded an attachment but did not send the final message
    When the adapter reports partial success
    Then the connector should record recovery-required delivery state
    And a later worker should not repeat the same action without reconciliation or operator recovery

  Scenario: External identities do not grant kernel authority
    Given an external sender has an admin role in Feishu
    When the connector maps the event into a request context
    Then the request context may record the external role as origin metadata
    And it should not set kernel permission mode, sandbox profile, approval policy, credential authority, or memory authority
