Feature: Kernel session search

  Genesis exposes session search as a bounded read-only projection so shells can
  find prior sessions without rebuilding their own history indexes.

  Scenario: Search finds sessions through safe projection fields
    Given the ledger contains multiple sessions with user and assistant turns
    When a caller searches sessions with a non-empty query
    Then matching sessions are returned newest first
    And each result includes the session id, updated time, match fields, and a bounded snippet
    And each result omits raw event ids, operation ids, job ids, credential refs, and storage paths

  Scenario: Empty search is rejected
    Given the ledger contains sessions
    When a caller searches sessions with an empty query
    Then the request is rejected as invalid_request
    And the ordinary session list remains available through /sessions

  Scenario: Search is stable after restart
    Given a session search returns a matching session
    When the kernel restarts with the same ledger
    Then the same search returns the same session result

