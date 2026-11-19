Fixed
- With an admin key set, a write request (POST, PUT, PATCH or DELETE) that carries no key at all now
  answers 401 before the server reads its JSON body. Before, a malformed body drew a 400, and a valid
  one cost a full parse before the 401. A request that carries a key, right or wrong, is checked as
  before. With no admin key configured nothing changes (ginsys/shelly-manager#720).
