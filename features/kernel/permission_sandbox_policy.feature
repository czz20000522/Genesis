Feature: Permission and sandbox policy
  Genesis keeps user-facing trust modes separate from the kernel execution
  policy that admits effects and selects the sandbox profile.

  Scenario: Plan mode is read-only and does not ask for escalation
    Given Genesis is running in plan mode
    When the model requests an effectful shell action
    Then Genesis denies the effect before execution
    And the resolved authority policy is read-only
    And the resolved approval policy does not ask for escalation

  Scenario: Default mode allows governed workspace effects only
    Given Genesis is running in default mode with a workspace root
    When the model requests a supported workspace shell action
    Then Genesis admits the action through the governed workspace executor
    And the resolved authority policy allows workspace writes
    And the resolved sandbox profile does not claim unrestricted host access

  Scenario: Yolo mode uses full host access with audit bounds
    Given Genesis is running in yolo mode
    When the model requests an effectful shell action
    Then Genesis may execute it through the host shell
    And the resolved authority policy allows full access
    And Genesis still records audit evidence and bounded output

  Scenario: Unavailable stronger sandbox profiles fail closed
    Given Genesis is configured with a stronger workspace OS sandbox profile
    And that sandbox profile is not available to the current executor
    When the model requests an effectful shell action
    Then Genesis denies the effect before execution
    And Genesis returns sandbox profile feedback to the model
    And Genesis records blocked operation evidence without degrading to host execution

  Scenario: Approval-required write tools do not run without an approval owner
    Given Genesis is configured with approval policy on request
    And no approval owner has granted the requested write effect
    When the model requests an effectful shell action
    Then Genesis denies the effect before execution
    And Genesis returns approval required feedback to the model
    And read-only kernel control tools remain admissible
