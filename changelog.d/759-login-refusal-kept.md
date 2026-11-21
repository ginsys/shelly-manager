Fixed
- A device that refuses the saved login is now reported as a refused login even when the identity
  check that follows it times out. Status, energy and the current configuration reads answer 502
  with the message that names the credentials, where they said "did not answer" or a bare 500
  before. An identity mismatch keeps its own answer. Fixes ginsys/shelly-manager#759.
