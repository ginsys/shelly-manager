Fixed

- A drift schedule now carries a revision that every edit, toggle and disable bumps, and the scheduler claims a tick only at the revision it read. A tick read before a change of the device selection, or before a disable and enable, is no longer run from the old read. An edit costs the tick in flight one claim; whether the schedule then runs on the next poll or at the next instant of its spec depends on whether the edit recomputed its next run. Refs ginsys/shelly-manager#279.
