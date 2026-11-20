Fixed

- The device detail page has one control that opens the configuration page instead of two ("View Full Config" is gone, "Edit Configuration" stays). The drift status now says why it is empty: "No desired configuration yet: add a template or overrides" for a device with no desired configuration, and "Could not check drift" with the reason when the check fails. The drift check waits as long as the server does for the device. Fixes ginsys/shelly-manager#756.
