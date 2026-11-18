Fixed
- The provisioner agent can now register with the manager and read its answers. `POST
  /api/v1/provisioner/agents/register` was the one route that answered without the `{success, data}`
  envelope, and the agent's client read every answer from the top level, so registration failed with
  "agent registration failed" and a poll showed no tasks. The register route now uses the envelope, and
  the client reads `data`. The agent also registers again before a poll when its start-up registration
  failed or the manager lost it in a restart, instead of failing until it was restarted. A test now runs
  the real client against the real router. Fixes ginsys/shelly-manager#627.
