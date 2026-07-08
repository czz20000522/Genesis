Feature: Agent invocation admission

  Genesis records bounded model-backed invocation authority before any child
  or delegated run can execute.

  Scenario: Root invocation admits a policy-allowed grant
    Given Genesis has a write-enabled tool policy
    When an application admits an agent invocation with a workspace_edit grant
    Then Genesis records an agent_invocation.admitted fact
    And the invocation projection includes the requested tool grant
    And the projection omits sandbox profiles, provider routes, and credentials

  Scenario: Role labels do not grant write tools
    Given Genesis is configured in plan permission mode
    When an application admits an agent invocation with role reviewer and a workspace_edit grant
    Then Genesis refuses the grant as capability_grant_tool_not_allowed
    And no invocation fact is recorded

  Scenario: Child invocation cannot exceed parent grant
    Given a parent invocation was admitted with only resource_read
    When an application admits a child invocation with workspace_edit
    Then Genesis refuses the child as capability_grant_exceeds_parent
    And no child invocation fact is recorded
