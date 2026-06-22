Feature: Tool result taxonomy
  Genesis lets the model read tool outcomes like a governed terminal while
  keeping kernel admission, policy, and infrastructure failures distinct.

  Scenario: Invalid tool requests are repair feedback
    Given the model requests a tool with invalid arguments
    When Genesis can still correlate the tool request
    Then Genesis returns tool request invalid feedback to the model
    And no tool effect is executed
    And the feedback is structured enough for the model to repair the request

  Scenario: Permission denials do not execute effects
    Given the model requests a structurally valid effectful tool call
    And the current permission policy denies the effect
    When Genesis handles the request
    Then Genesis returns permission denied feedback to the model
    And no command effect occurs
    And inspection can show that the operation was blocked by policy

  Scenario: Nonzero command exits are command results
    Given the model requests an admitted shell command
    When the command runs and exits nonzero
    Then Genesis returns the exit code, stdout, and stderr as observed command evidence
    And Genesis does not reinterpret the command failure as a kernel failure

  Scenario: Tool infrastructure failures are not command stderr
    Given the model requests an admitted tool call
    When the shell runtime, ledger, or tool runtime infrastructure fails
    Then Genesis reports a tool infrastructure failure
    And Genesis does not disguise it as command stdout, command stderr, or a normal command exit

  Scenario: Long command output stays bounded and inspectable
    Given an admitted shell command produces long stdout or stderr
    When Genesis returns the tool result
    Then the visible output is bounded with a head and tail policy
    And the result includes truncation metadata
    And inspection can distinguish bounded display from observed command evidence
