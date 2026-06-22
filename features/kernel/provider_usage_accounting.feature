Feature: Provider usage accounting
  Genesis relies on provider-reported usage for model token facts and records
  those facts for inspection and context management.

  Background:
    Given a configured provider returns usage for a model exchange

  Scenario: Normalizing provider token usage
    When the provider returns input, output, total, cache hit, and cache miss token counts
    Then Genesis records those counts as normalized provider usage evidence
    And Genesis does not compute those counts from local text length

  Scenario: Recording model context accounting
    Given a provider request includes conversation history and current user input
    When the provider returns usage for that request
    Then Genesis records model context accounting for that exchange
    And the accounting identifies the model input categories that were sent
    And the accounting identifies the completed history turns that were included

  Scenario: Cache misses are processed input evidence
    When the provider reports cache miss input tokens
    Then Genesis records those tokens as provider-backed processed input tokens
    And context compaction may consume that value as accounting evidence
    And Model Gateway does not execute compaction because it recorded the usage

  Scenario: Request usage is not fragment attribution
    When a provider response reports input tokens for a request
    Then Genesis treats the count as request-level evidence
    And Genesis does not claim per-message, per-fragment, or per-turn token attribution from that field alone
