Feature: Provider reasoning messages

  Scenario: A local adapter preserves reasoning with a final answer
    Given a configured provider adapter returns reasoning and visible final text
    When Genesis completes the turn
    Then the session contains one durable reasoning message and one final assistant message
    And the timeline projects the reasoning separately from final text after restart

  Scenario: An adapter omits reasoning for an ordinary continuation
    Given a persisted reasoning message whose selected adapter disposition is omitted
    When the user submits a later turn through that adapter
    Then the adapter request contains prior final assistant text in its required position
    And it does not contain the prior reasoning field
    And the user can still read the prior reasoning in the session timeline

  Scenario: A DeepSeek tool continuation keeps reasoning response-only
    Given a persisted DeepSeek reasoning message with tool calls
    When the tool result continues the turn through the same adapter binding
    Then the adapter request does not contain the prior reasoning field
    And the assistant tool-call message contains empty content and native tool calls

  Scenario: GLM preserved thinking replays only through a same-binding tool continuation
    Given a persisted zai-glm glm-5.2 reasoning message with tool calls
    When its tool result continues the turn through the same adapter binding
    Then the request sends thinking clear_thinking false
    And it places the unchanged reasoning before the assistant tool calls
    But an ordinary later user turn sends clear_thinking true without prior reasoning

  Scenario: GLM preserved thinking refuses an unavailable tool continuation
    Given a zai-glm glm-5.2 tool continuation has missing or differently bound reasoning
    When Genesis prepares the continuation request
    Then Genesis returns provider_reasoning_continuation_unavailable
    And it does not contact the provider
