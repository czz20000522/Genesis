Feature: Workspace edit tool

  Genesis exposes a typed, workspace-confined edit primitive for
  model-initiated file changes.

  Scenario: Exact replacement mutates one workspace file
    Given Genesis is configured with a workspace root and write-enabled tool policy
    And a file inside the workspace contains the old string exactly once
    When the model calls workspace_edit with that relative path and replacement
    Then Genesis updates the file once
    And the tool result reports a bounded completed edit
    And the tool result omits the workspace root and host absolute path

  Scenario: Read-only permission mode blocks edits
    Given Genesis is configured in plan permission mode
    And a file inside the workspace contains the old string exactly once
    When the model calls workspace_edit
    Then Genesis returns permission_denied
    And the file remains unchanged

  Scenario: Workspace confinement rejects escape paths
    Given Genesis is configured with a workspace root and write-enabled tool policy
    When the model calls workspace_edit with a path outside the workspace
    Then Genesis returns repairable tool_request_invalid feedback
    And no file outside the workspace is modified

  Scenario: Ambiguous old string does not mutate
    Given Genesis is configured with a workspace root and write-enabled tool policy
    And a file inside the workspace contains the old string more than once
    When the model calls workspace_edit
    Then Genesis returns workspace_edit_old_string_not_unique
    And the file remains unchanged
