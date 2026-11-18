Security
- The provisioner agent routes (`/api/v1/provisioner/*`), `POST /api/v1/discover` and
  `POST /api/v1/provisioning/provision` now refuse a request without the admin key when one is set;
  the task poll returned device credentials and Wi-Fi settings to anyone. The kustomize DaemonSet
  gives the agent the key from the `shelly-manager-secrets` Secret through `secretKeyRef`. A route
  table test now fails on any route that answers without the key and is not listed; 29 such routes
  are listed (ginsys/shelly-manager#611). With no admin key configured the routes stay open, as
  before. Rotating the admin key through the API breaks every agent until the Secret is updated
  (ginsys/shelly-manager#610). Fixes ginsys/shelly-manager#607.
