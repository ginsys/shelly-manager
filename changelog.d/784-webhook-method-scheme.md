Fixed

- Creating or updating a notification channel now refuses a webhook method other than POST, PUT or PATCH and a webhook or Slack URL that is not an absolute http or https URL with a host, with the rule as the 400 reason. Before, such a channel was stored and failed at every delivery. The reason never carries the URL, and a save that leaves the stored config unchanged is not checked again, so a channel stored earlier can still be disabled; it keeps failing at delivery until its method or URL is fixed. Refs ginsys/shelly-manager#784.
