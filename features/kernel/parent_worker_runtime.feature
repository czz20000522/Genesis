Feature: Parent-led worker runtime configuration

  Genesis binds worker roles to model profiles and preset tool sets before a
  parent can create bounded worker invocations.

  Scenario: Role binding projection exposes preset tools and model profile
    Given models.json defines a worker role bound to a model profile and preset tools
    When Genesis reads the parent-worker runtime bindings
    Then the worker role projection includes the role id, profile id, model id, provider route, preset tools, and max_parallel
    And the projection omits credentials, raw prompts, sandbox profiles, and permission profiles

  Scenario: Same role can have multiple worker instances
    Given a worker role binding has max_parallel 2
    When a parent admits two worker invocations for the same role
    Then Genesis treats them as separate invocation identities
    And both invocations use the same preset tool set

  Scenario: Parent and role concurrency limits are configuration-driven
    Given a worker role omits max_parallel and a parent binding omits max_children
    When Genesis projects the parent-worker runtime bindings
    Then the role max_parallel is 6 and the parent max_children is 24
    And an explicit parent max_children limits active workers across its allowed roles

  Scenario: Parent cannot add tools at invocation time
    Given a worker role binding only presets resource_read
    When a parent tries to create that worker with workspace_edit
    Then Genesis refuses the worker as capability_grant_exceeds_role
    And no worker invocation fact is recorded

  Scenario: Task graph is not owned by the parent-worker runtime
    Given a worker invocation has a role id and terminal result
    When a future task graph references that invocation
    Then the parent-worker runtime provides invocation identity and child conversation projection only
    And graph nodes, edges, layout, and dependency scheduling are governed by the task graph requirement

  Scenario: Worker output is readable as a child conversation
    Given a worker invocation was admitted and completed with a final answer
    When an application reads the child conversation projection
    Then Genesis returns the worker role, status, final answer, usage, context scope, tool set, and evidence refs
    And the projection omits the focused prompt, raw provider stream, raw tool trace, credentials, sandbox profiles, and permission profiles
    And the parent session transcript remains separate

  Scenario: Parent dispatches a role-bound leaf worker
    Given the parent tool manifest exposes delegate_worker
    When the parent calls delegate_worker with a configured role id and focused task
    Then Genesis resolves provider, model profile, tools, and context policy from the role binding
    And the worker does not receive delegate_worker or a parent-history fork
    And the parent receives only the worker terminal result, usage, and evidence summary
