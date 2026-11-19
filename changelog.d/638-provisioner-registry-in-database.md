Changed
- The provisioner registry (agents, their tokens and the tasks) now lives in the database instead of the
  manager's memory, so a manager restart keeps it and every replica sees the same registry. A token is
  stored only as a hash. An agent id, a task type, a task status or a task agent id longer than 191
  characters is now refused with 400. A poll that finds no task answers an empty list instead of null. An agent
  that was silent for more than 5 minutes and then reports devices is listed online again, and cancelling
  a task no longer changes a sibling task that a bulk create had given the same configuration.
  On MySQL the two new tables use a binary collation, which needs MySQL 8.0.17 or later; on MariaDB or an
  older MySQL the manager logs a warning at start-up and keeps the default collation. Refs
  ginsys/shelly-manager#638.
