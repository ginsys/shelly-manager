Fixed
- A SQLite DSN parameter the driver refuses, such as `_txlock` or `_time_format`, is now named in the connect error without its value. The driver quoted the value, so a mistyped secret in the DSN query could reach the error and the log. Fixes ginsys/shelly-manager#596.
