Feature: Kernel owner boundaries
  Genesis remains a generic agent kernel rather than an application-specific
  bot, shell, or integration bundle.

  Scenario: External channels submit ordinary turns
    Given an external daemon receives a message from a configured channel
    When the daemon forwards the message to Genesis
    Then Genesis admits it as a normal turn request
    And the daemon does not own session lifecycle, model choice, memory, or tool authority
    And the kernel does not learn channel-specific reply APIs

  Scenario: Outbound application actions stay in user-space capabilities
    Given an installed skill describes how to use an external CLI
    When the model needs to act through that external application
    Then Genesis exposes only the generic governed tool path
    And the skill or CLI remains outside the kernel
    And no application-specific command alias is added to the kernel

  Scenario: Skill discovery is metadata-first
    Given configured skill roots contain installed skills
    When Genesis builds model context for a turn
    Then the provider context includes a bounded skill index
    And the provider context does not include full skill bodies by default
    And protected inspection surfaces do not expose skill paths or bodies

  Scenario: Shells and applications request kernel behavior through commands
    Given a shell or application wants context compaction
    When it asks Genesis to compact a session
    Then it submits a typed kernel command
    And only the kernel compaction runner summarizes, truncates, and records compaction evidence
