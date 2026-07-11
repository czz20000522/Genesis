Feature: Desktop provider control
  The desktop remains a shell over provider configuration and kernel execution
  owners while allowing a local operator to manage configured profiles.

  Scenario: Inspect configured profiles without exposing credentials
    Given Genesis Home contains local and cloud model profiles
    When the operator opens the desktop provider panel
    Then the panel shows profile, model, route, adapter, role binding, and credential-present state
    And it does not show an API key, credential path, or provider command arguments

  Scenario: Rotate a cloud credential as a one-shot entry
    Given a configured cloud profile has a local credential reference
    When the operator submits a replacement API key from the desktop provider panel
    Then the local credential store contains a protected credential record
    And the panel receives only credential-present state
    And the API key is not stored in desktop browser storage or kernel events

  Scenario: Verify and activate a configured profile
    Given the operator selects a configured profile for a role
    When the operator requests verification
    Then genesisd verifies that selection through its configured adapter
    When the operator applies the selection with an owned kernel and no active turn
    Then only the owned genesisd sidecar is restarted
    And settled sessions remain available after readiness returns

  Scenario: Preserve external-kernel ownership
    Given the desktop is attached to an external kernel
    When the operator applies a configured profile selection
    Then the binding is saved locally
    And the desktop reports that an external restart is required
    And it does not start, stop, or restart the external kernel
