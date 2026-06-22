Feature: Kernel context compaction
  Genesis keeps long-running sessions usable by compacting provider context
  through a kernel-owned command and evidence trail.

  Background:
    Given a session has completed conversation turns
    And provider usage is available for the current exchange

  Scenario: Automatic compaction is triggered by provider input pressure
    Given the configured context window has an auto-compaction threshold
    When provider-reported input tokens cross that threshold
    Then Genesis submits an automatic context compaction command
    And the kernel compaction runner records compaction started evidence
    And no shell, provider adapter, or external daemon performs the compaction

  Scenario: Compaction changes model context without rewriting user history
    Given context compaction has completed through an older turn
    When Genesis builds provider context for a later turn
    Then the provider context contains the compacted summary
    And the provider context keeps the configured recent complete turns verbatim
    And the provider context excludes raw turns already covered by the summary
    And the user-facing session history still contains the original turns

  Scenario: User-facing timelines hide internal summaries
    Given context compaction has completed
    When a user views the session timeline
    Then the timeline shows a compaction notice
    And it does not render the internal compaction summary as an assistant message

  Scenario: Token-budgeted tails use provider-backed accounting only
    Given recent tail token budgeting is configured
    And recent turns have provider-backed processed input token accounting
    When Genesis selects the recent tail after compaction
    Then it may keep additional complete recent turns within that budget
    And it stops expanding when provider-backed accounting is missing
    And it never estimates model tokens from local text length

  Scenario: Compaction failure is recoverable
    When the compaction summarizer fails
    Then Genesis records compaction failed evidence
    And the completed user turn remains completed
    And a later eligible turn may retry compaction
