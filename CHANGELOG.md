## 0.4.0 (2020-04-12)

- results command can now look up a PR ([#30](https://github.com/katbyte/tctest/issues/30))
- multiple PRs can now be specified 

## 0.4.0 (2020-04-12)

- support TeamCity token Auth ([#28](https://github.com/katbyte/tctest/issues/26))
- prevent test auto-detection running all tests unexpectedly ([#28](https://github.com/katbyte/tctest/issues/28))

## 0.3.1 (2020-04-12)

- Update TeamCity API endpoints to `/app/rest/2018.1` ([#21](https://github.com/katbyte/tctest/issues/21))

## 0.3.0 (2020-02-29)

- Prevent `_test_test.go` PR file lookup ([#20](https://github.com/katbyte/tctest/issues/20))
- Rename `status` to `results` for accuracy ([#15](https://github.com/katbyte/tctest/issues/15))
- Rename root command to `branch` to avoid triggering accidental builds on typos ([#15](https://github.com/katbyte/tctest/issues/15))
- Add `--wait` option to `pr`, `branch`, and `results` commands ([#15](https://github.com/katbyte/tctest/issues/15))
- `list` command no longer triggers a build when no tests are found ([#15](https://github.com/katbyte/tctest/issues/15))
- Usage information is no longer displayed after non-usage-related errors ([#15](https://github.com/katbyte/tctest/issues/15))
- `results` command will display a warning if an in-progress build would give incomplete results ([#15](https://github.com/katbyte/tctest/issues/15))
- `results` will now inform the user if the specified build is still queued ([#15](https://github.com/katbyte/tctest/issues/15))

## 0.2.0 (2020-01-22)

- support `azurerm` new package per service structure
- new command `status` to get tests results ([#5](https://github.com/katbyte/tctest/issues/5))

## 0.1.0 (2019-05-24)

Initial release!
