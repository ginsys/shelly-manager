Fixed
- The GitOps export no longer copies the request configuration into `export-summary.yaml`: a secret in the request, such as a token, reached a file that normally goes into a git repository. The file keeps its export metadata, structure counts and generation time. Fixes ginsys/shelly-manager#737.
