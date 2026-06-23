Feature: Kernel and user-space application boundary

  Genesis keeps application domains outside the kernel unless they become
  generic kernel primitives.

  Scenario: A calculator skill remains user-space
    Given a calculator package contains instructions for exact arithmetic
    When an application submits a calculation turn
    Then the kernel may provide model context and governed tools
    And the calculator package must not become a kernel tool or owner

  Scenario: A Feishu daemon remains user-space
    Given a Feishu daemon receives an external message
    When it forwards the message to Genesis
    Then it submits a kernel turn command
    And it must not assemble provider context or write ledger truth itself

  Scenario: A shell cannot own model context
    Given WebUI, CLI, or a desktop shell has prior conversation text
    When it submits another user turn
    Then the Model Gateway rebuilds provider-visible context from kernel state
    And the shell must not send its own hidden conversation history to the provider

  Scenario: Applications cannot write memory truth
    Given an application observes a user preference
    When the preference should become durable memory
    Then the application submits or causes a memory candidate through the kernel path
    And approved recall must still depend on kernel-owned review state
