Security
- The provisioner agent now has its own key, `security.agent_api_key`
  (`SHELLY_SECURITY_AGENT_API_KEY`). The manager accepts it next to the admin key on the five agent
  routes only: register, poll tasks, update task status, report discovered devices and health. Creating
  tasks and listing agents, tasks or discovered devices still need the admin key, and the agent key
  opens no other route. The kustomize DaemonSet reads it from the `agent-api-key` key of its Secret, so
  a leaked agent key no longer exposes the admin key, and rotating the admin key through the API no
  longer breaks the agents. The admin key is now compared in constant time on every guard that checks
  it, and the `Bearer ` prefix stays case-sensitive. Migration: set `agent-api-key` in
  `shelly-manager-secrets` to a value different from the admin key, then restart the manager and the
  DaemonSet; until then the agents get 401 when an admin key is set. Fixes ginsys/shelly-manager#610.
