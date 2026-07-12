Feature: Session model binding

  Scenario: Sessions retain independent selected models
    Given session A is bound to profile "deepseek-flash"
    And session B is bound to profile "local-qwen"
    When Genesis restarts
    Then session A projects "deepseek-flash"
    And session B projects "local-qwen"

  Scenario: A running session cannot switch models
    Given session A has an active turn
    When its operator selects a different model profile
    Then Genesis refuses the model change without writing a binding fact
