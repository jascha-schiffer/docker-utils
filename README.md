# docker-utils

quick and dirty utility collection

```bash
> docker-utils service wait --help

Usage:  docker-utils service wait [OPTIONS]

Wait for service replication

Options:
  -f, --filter filter       Filter output based on conditions provided
      --format string       Pretty-print services using a Go template
      --interval duration   Interval in which we check the status (default 1m0s)
  -q, --quiet               Only display IDs
      --timeout duration    Max duration to wait (default 10m0s)

```