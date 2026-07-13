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

  Scenario: Project planning can evolve without rewriting completed evidence
    Given a graph contains unstarted task nodes and a pending dependency edge
    When its owner refines a task or removes that pending edge
    Then Genesis persists the changed topology as new graph facts
    And completed, failed, and cancelled task evidence cannot be changed

  Scenario: Parent explicitly links an admitted worker without graph dispatch
    Given a graph node is ready and a parent has separately admitted a worker invocation
    When the parent binds that existing invocation to the ready node
    Then Genesis records the opaque invocation link without starting a worker
    And the parent cannot bind or edit a graph from another session
    And the same invocation cannot be linked to a second node
    And an owner running fact makes the linked node immutable
    And a terminal invocation result reduces the node with durable evidence
    And restart reconciles a missed graph reduction from that durable result without replay
    And the latest ambiguous invocation run after restart becomes failed without replay
