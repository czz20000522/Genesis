Feature: Kernel sandbox readiness and approval owner command path
  Genesis Kernel must keep sandbox enforceability, approval ownership, and shell/control surfaces separate.

  Background:
    Given a write-effect tool request has passed schema validation
    And the kernel has resolved permission mode, sandbox profile, approval policy, workspace root, and executor adapter

  Scenario: Unavailable sandbox blocks before approval
    Given the resolved sandbox profile is os_workspace
    And the current executor cannot enforce os_workspace
    When the tool request is admitted
    Then the kernel records sandbox readiness evidence as unavailable
    And the tool effect is not executed
    And no approval request can downgrade the effect to host execution

  Scenario: Approval required records a frozen effect request
    Given sandbox readiness is available
    And approval policy is on_request
    When the write-effect tool request is admitted
    Then the kernel records approval.requested
    And the approval binds the original tool call or operation ref
    And the approval binds the resolved policy snapshot
    And the approval binds the requested effect summary
    And the tool effect is not executed
    And the model-visible tool result contains only repairable approval_required feedback

  Scenario: Approval allows the original frozen effect
    Given there is a pending approval for a frozen effect request
    When an operator submits an approve decision through the kernel command path
    Then the kernel validates the approval id, authority, evidence, expiry, and current policy snapshot
    And the kernel records approval.approved before executing the effect
    And the effect executes under the resolved sandbox profile from the frozen request
    And the model does not need to submit another tool call

  Scenario: Denial and stale approval decisions fail closed
    Given there is a pending approval for a frozen effect request
    When an operator denies the approval
    Then the kernel records approval.denied
    And the kernel records terminal blocked operation evidence
    And the effect is not executed
    When an unknown, expired, mismatched, or stale approval decision is submitted
    Then the kernel rejects the command fail-closed
    And no effect is executed

  Scenario: Approval surface is only a control surface
    Given a pending approval exists
    When a CLI, console, HTTP client, or future UI lists and decides the approval
    Then the surface only submits an approval decision command
    And it does not write tool.result
    And it does not decide permission, sandbox, workspace, credentials, or policy locally
