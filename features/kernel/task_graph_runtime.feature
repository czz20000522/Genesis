Feature: Ledger-owned TaskGraph

  Scenario: A graph accepts only a valid dependency DAG
    Given a graph has admitted node references
    When its owner receives a cyclic or duplicate dependency edge
    Then Genesis rejects the proposal without recording a graph fact

  Scenario: A dependency controls readiness without granting authority
    Given a node depends on another node
    When the predecessor is completed successfully
    Then the dependent node becomes ready
    And its referenced execution owner retains provider and tool authority
