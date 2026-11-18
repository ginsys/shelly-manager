Fixed
- The provisioner DaemonSet in the kustomize tree now reaches the manager. It set no
  `SHELLY_API_URL`, so it fell back to `http://0.0.0.0:8080`, and being on the host network it also
  could not resolve a Service name. Each overlay now sets `SHELLY_API_URL` to its own Service
  (`http://dev-shelly-manager:80`, `staging-` and `prod-` likewise), the DaemonSet reads that one
  key from `shelly-manager-config`, and `dnsPolicy` is `ClusterFirstWithHostNet`. An overlay that
  sets none leaves the pod unable to start instead of dialling a default.
  Fixes ginsys/shelly-manager#591.
