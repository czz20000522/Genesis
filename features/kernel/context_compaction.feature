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
    And Genesis may defer the immediate retry with bounded backoff evidence
    And a later eligible turn may retry compaction

  Scenario: Compaction source preserves completed tool outcomes
    Given a completed turn contains a model tool call and its matching tool result
    When Genesis compacts that completed turn
    Then the compaction source includes the tool call before the assistant answer
    And the compaction source includes the matching tool result
    And the compaction source does not expose kernel event ids or operation ids as summary content

  Scenario: Completed compaction records provider-backed economics
    Given compacted turns have provider cache hit and cache miss usage evidence
    When context compaction completes
    Then Genesis records the provider usage that triggered compaction
    And Genesis records cache-stability metrics for the compacted region
    And those metrics do not claim per-message token attribution
