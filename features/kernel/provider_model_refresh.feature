Feature: Provider model refresh

  Genesis refreshes provider model catalogs only when an operator explicitly
  asks for it, then uses the local catalog for later model binding.

  Scenario: Manual refresh persists a provider model catalog
    Given models.json contains an OpenAI-compatible provider route with a local credential ref
    And the provider /models endpoint returns duplicate model ids
    When the operator manually refreshes the provider model catalog
    Then Genesis writes a sorted de-duplicated catalog snapshot to models.json
    And the active model profile binding is unchanged
    And the refresh output omits API keys, credential refs, headers, and local secret paths

  Scenario: Failed refresh preserves the prior catalog
    Given models.json already contains a provider model catalog
    And the provider /models endpoint fails
    When the operator manually refreshes the provider model catalog
    Then Genesis reports a sanitized refresh failure
    And the previous catalog remains unchanged
    And the active model profile binding is unchanged

  Scenario: Provider command routes are not probed in Phase A
    Given models.json selects a provider_command route
    When the operator manually refreshes the provider model catalog
    Then Genesis reports provider_model_refresh_unsupported
    And no provider command process is started
