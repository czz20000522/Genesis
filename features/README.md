# Genesis Kernel Feature Specs

This directory contains BDD feature files for Genesis Kernel behavior.
They are acceptance contracts first and executable test inputs second.

The first feature files are intentionally focused on kernel behavior that
must stay stable while the implementation changes:

- provider-owned token usage accounting;
- kernel-owned context compaction;
- terminal-equivalent tool results;
- kernel boundaries between core, shells, daemons, skills, and apps.

## Writing Rules

- Describe observable behavior in Genesis domain language.
- Keep scenarios independent and focused on one rule.
- Drive future automation through public kernel commands and projections.
- Do not bind scenarios to private helper names, storage files, or UI copy.
- Do not add application-specific integrations as kernel features.
- Do not encode retired concepts as active expectations.

Step definitions are not wired yet. Until they are, these files are the
reviewable behavior source that guides implementation and later automation.
