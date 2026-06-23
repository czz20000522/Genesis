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

  Scenario: Foreground shell interruption is not an ordinary command failure
    Given the model requests an admitted foreground shell command
    When the user interrupts the active turn while that command is still running
    Then Genesis records an interrupted operation result
    And Genesis returns an interrupted tool result for the original tool call
    And Genesis records an assistant interruption fact instead of a model final answer

  Scenario: Turn interruption does not cancel existing managed jobs
    Given a session already has a running Genesis-managed job
    When the user interrupts a later active provider turn in the same session
    Then Genesis cancels the active provider step
    And Genesis records an assistant interruption fact
    And the existing managed job remains running unless explicit job cancellation is requested

  Scenario: Long command output stays bounded and inspectable
    Given an admitted shell command produces long stdout or stderr
    When Genesis returns the tool result
    Then the visible output is bounded with a head and tail policy
    And the result includes truncation metadata
    And inspection can distinguish bounded display from observed command evidence
