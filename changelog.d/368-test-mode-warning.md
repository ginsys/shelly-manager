Added
- The server now logs a `WARN` at startup when test mode is on. With
  `SHELLY_SECURITY_VALIDATION_TEST_MODE=true` it names the protections the test-mode router leaves out
  (rate limiting, request validation and the rest); with `security.validation_test_mode`
  alone it names the two request checks that are skipped. Nothing it turns off changes. Fixes
  ginsys/shelly-manager#368.
