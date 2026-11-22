Fixed
- The published manager image now serves the web UI at `/`: the image build compiles `ui/` in a Node stage and copies `ui/dist` into the final image, where it used to show only the minimal landing page. Refs ginsys/shelly-manager#846.
