## 0.3.0 (Unreleased)

- Prevent _test_test.go PR file lookup ([#20](https://github.com/katbyte/tctest/issues/20))
- Rename `status` to `results` for accuracy ([#15](https://github.com/katbyte/tctest/issues/15))
- Rename root command to `branch` to avoid triggering accidental builds on typos ([#15](https://github.com/katbyte/tctest/issues/15))
- Add `--wait` option to `pr`, `branch`, and `results` commands ([#15](https://github.com/katbyte/tctest/issues/15))
- `list` command no longer triggers a build when no tests are found ([#15](https://github.com/katbyte/tctest/issues/15))
- Usage information is no longer displayed after non-usage-related errors ([#15](https://github.com/katbyte/tctest/issues/15))
- `results` command will display a warning if an in-progress build would give incomplete results ([#15](https://github.com/katbyte/tctest/issues/15))
- `results` will now inform the user if the specified build is still queued ([#15](https://github.com/katbyte/tctest/issues/15))

## 0.2.0 (Jan 22th, 2020)

- support `azurerm` new package per service structure
- new command `status` to get tests results ([#5](https://github.com/katbyte/tctest/issues/5))

## 0.1.0 (May 24th, 2019)

Initial release!
