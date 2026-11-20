Added
- `DELETE /api/v1/provisioner/agents/{id}` (admin key) frees an agent's id and revokes its token, and fails
  the agent's assigned tasks. The manager also forgets by itself: a register starts a purge, at most once
  an hour, that removes agents silent for 30 days with no pending task and deletes completed and failed
  tasks last updated more than 30 days ago. Finished tasks now stay until a purge removes them, where a
  restart used to clear every task. Refs ginsys/shelly-manager#638.
