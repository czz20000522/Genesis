Feature: Persistent session workspace binding

  Scenario: A Project session supplies its selected directory as default workspace
    Given a desktop session is bound to an existing Project directory
    When the model runs a workspace tool without a cwd
    Then Genesis uses the bound Project directory as the default cwd
    And a second Project session may bind the same directory with independent history

  Scenario: A Task session has an independent durable workspace
    Given two desktop Task sessions receive different directories below the Genesis task root
    When either Task session uses its default workspace
    Then it does not read or write through the other Task session's default root
    And both bindings remain after restart

  Scenario: A Chat session has no implicit workspace
    Given a desktop Chat session is bound with mode none
    When the model requests a filesystem operation without a cwd
    Then Genesis does not invent a working directory
    And the chat transcript remains available after restart

  Scenario: Permission mode governs cross-workspace access
    Given a session has a primary Project workspace and the model names another directory
    When the operation is read-only in plan or default mode
    Then Genesis permits the explicit readable directory
    But a default-mode write outside the primary workspace is refused
    And yolo may perform the explicit host-level write
