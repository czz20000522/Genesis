Feature: User-space message ingress runtime
  External channel messages can enter Genesis Kernel without making the channel a kernel owner.

  Scenario: A channel message submits one kernel turn
    Given a ChannelMessage from channel "feishu" adapter "feishu-inbound"
    And the message id is "msg-1"
    And the thread id is "chat-1"
    And the text is "hello"
    When the user-space message ingress runtime processes the message
    Then it should map the channel thread to an opaque kernel session
    And it should submit one turn to the kernel
    And it should not send an external-channel reply itself

  Scenario: Duplicate inbound delivery does not execute another turn
    Given a ChannelMessage from channel "feishu" adapter "feishu-inbound"
    And the message id is "msg-1"
    When the runtime has already processed that message id
    And the same message is processed again
    Then it should not submit another kernel turn
    And it should return the existing application submission record

  Scenario: Inbound context gives the LLM a reply reference without granting authority
    Given a valid Feishu ChannelMessage with chat id "oc_123"
    When the runtime submits the message to the kernel
    Then the turn input should include the source channel and chat id as inbound context
    And it should not set permission mode, sandbox profile, approval policy, credential authority, or provider context

  Scenario: Outbound channel actions are performed by LLM skill and tools
    Given a Feishu message enters Genesis through ingress
    When the LLM decides a Feishu reply is needed
    Then the LLM should use skill instructions and kernel-governed shell tools to call lark-cli
    And the ingress runtime should not provide a gateway reply API
