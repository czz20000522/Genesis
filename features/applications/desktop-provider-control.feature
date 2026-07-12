Feature: Desktop provider control

  Scenario: A user configures DeepSeek Flash from an empty Genesis Home
    Given Genesis Home has no model profiles
    When the user submits a DeepSeek Flash API key from the desktop
    Then Genesis Home contains the DeepSeek Flash profile without the API key
    And the desktop can verify the profile before applying it to coordinator
