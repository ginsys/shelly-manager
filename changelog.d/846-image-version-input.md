Added

- The image build can now be called with a release version (the `version` input of the `docker-build.yml` workflow when another workflow calls it): both images get that version as a tag beside the date name, and both binaries report it. A release build does not move `latest`, which stays the newest main image. With no version the build is the one it has always been, and a binary built that way reports `dev`. Refs ginsys/shelly-manager#846.
