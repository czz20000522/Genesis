Feature: Material source snapshots
  Genesis turns local or uploaded material into governed source snapshots so the
  model can inspect code packages without receiving host paths or storage refs.

  Scenario: A realistic code package fits the default source intake budget
    Given an operator submits a local zip code package through material intake
    And the package is larger than the old test-sized total budget
    When the source owner admits the package
    Then the admission returns a source snapshot reference
    And each source_read result remains bounded by the source read projection budget
    And capabilities expose the effective source snapshot intake and read limits

  Scenario: Source read tools stay safe for pure-read concurrent execution
    Given a source snapshot has admitted file handles at intake
    When source_tree and source_read run in the same pure-read execution window
    Then source_tree does not mutate source resolver state
    And source_tree does not expose newly added archive entries without a new intake
    And source_read continues to require an admitted source file reference

  Scenario: Source snapshot persistence is truthfully reported
    Given an uploaded zip has been admitted as a source snapshot
    When the kernel restarts with the same ledger and material store
    Then the old source snapshot reference is not silently recovered
    And capabilities report source snapshot persistence as process-lifetime only
    And provider context does not receive host paths or storage paths as a fallback
