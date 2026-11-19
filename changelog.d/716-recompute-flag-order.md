Fixed
- Saving a template or a device override now stores the new desired configuration and marks the device pending in one database write. Before, a failure of the second write, or an apply finishing between the two, could leave a device showing as applied against a configuration it never received. Fixes ginsys/shelly-manager#716.
