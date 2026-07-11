Feature: Stable provider context prefix

  Scenario: Stable instructions do not become user text
    Given a session has a role policy and an indexed skill catalog
    When Genesis prepares a new model request
    Then the system prefix contains the stable instructions and skill index
    And the current user message is a separate final user message

  Scenario: Native tools remain outside conversation text
    Given a session exposes a stable tool manifest
    When Genesis projects its request through an adapter
    Then the adapter receives the normalized tool manifest in its native tool field
    And no tool schema is concatenated into the current user message

  Scenario: Compaction preserves the prefix
    Given a session has enough variable conversation history to compact
    When Genesis replaces older history with a summary
    Then the stable prefix fingerprint is unchanged
    And the summary precedes the retained variable tail and current user message

  Scenario: Prefix changes are explained without exposing prefix text
    Given a session has a prior accounted provider prefix
    When its next request changes one stable prefix component
    Then context inspection lists the changed component name
    And it does not expose the corresponding raw system, skill, tool, or adapter text

  Scenario: A provider adapter preserves canonical role order
    Given a canonical conversation with system user assistant and tool messages
    When the llama.cpp provider command prepares its native request
    Then it preserves those message roles and order
    And it does not flatten them into one user message
