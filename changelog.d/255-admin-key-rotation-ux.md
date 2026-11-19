Changed
- The admin key rotation page takes the new key in two password fields and rotates only when both match, refuses a key that starts or ends with a space (the server could not match it again), says that a rotation lasts only until the server restarts, and carries test ids for end-to-end tests. Fixes ginsys/shelly-manager#255.
