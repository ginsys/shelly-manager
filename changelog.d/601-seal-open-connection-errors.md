Fixed
- Errors from an open database connection no longer carry driver text into the logs or the API.
  A rejected statement, a transaction that could not begin, a failed migration or `Close`, and the
  apply lock's queries used to quote the server address or socket path, identifiers and values;
  they now report `database operation failed: <kind>` with the SQLSTATE or MySQL error number
  where there is one. `gorm.ErrDuplicatedKey`, `gorm.ErrRecordNotFound` and context errors still
  answer `errors.Is`. Fixes ginsys/shelly-manager#601.
