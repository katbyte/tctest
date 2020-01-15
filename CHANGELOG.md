## 0.2.0 (Jan 15th, 2020)

- Rename `status` to `results` for accuracy
- Rename root command to `branch` to avoid triggering accidental builds on typos
- Add `--wait` option to `pr`, `branch`, and `results` commands
- `list` command no longer triggers a build when no tests are found
- Usage information is no longer displayed after non-usage-related errors
- `results` command will display a warning if an in-progress build would give incomplete results
- `results` will now inform the user if the specified build is still queued

## 0.1.0 (May 24th, 2019)

Initial release!