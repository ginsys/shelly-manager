Fixed
- A device whose stored desired configuration holds a section with nothing set in it, such as `{"system":{"sntp":{}}}`, is no longer refused by apply, and verify and the drift check no longer report that section as a difference. The section is skipped at any depth; a set `false`, `0` or `[]` is still a value. The stored configuration is not rewritten. Fixes ginsys/shelly-manager#863.
