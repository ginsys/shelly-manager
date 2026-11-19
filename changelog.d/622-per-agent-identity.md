Security
- Each provisioner agent now has its own identity. The manager issues an `agent_token` when an agent
  registers, and the agent sends it as `X-Agent-Token`. Poll tasks, update task status and report
  discovered devices act only for the token's agent: another agent's token is refused with 403, a missing
  or unknown token with 401 (the agent then registers again), and a report completes only its own agent's
  task. The shared agent key alone now opens register and health only; the admin key is unchanged and
  needs no token. Registering an id that is online (seen within 5 minutes) is refused with 409 unless the
  request carries that agent's token or the admin key. Migration: upgrade the manager and the agents
  together. Tokens are kept in memory, so after a manager restart the agents register again by
  themselves, and a restarted agent waits up to 5 minutes until its old record is silent. An agent from
  before this change cannot poll while an admin key is set. Fixes ginsys/shelly-manager#622.
