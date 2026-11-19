Fixed
- The export and import result routes (`GET /export/{id}`, `GET /export/backup/{id}` and `GET /import/{id}`) answer from the history record when the server no longer holds the result in memory, so a row the history lists no longer leads to "result not found" after a restart. The answer has the fields the record holds and leaves out the rest. Fixes ginsys/shelly-manager#647.
