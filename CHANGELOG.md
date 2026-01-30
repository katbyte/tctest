## v0.6.0 (Unreleased)

- add the `prs` sub-command ([#57](https://github.com/katbyte/tctest/issues/57))
- add new filter: title regex ([#73](https://github.com/katbyte/tctest/issues/73))

## v0.5.0 (2022-08-09)

- support detecting azurerm `services`
- support for Github OAUTH tokens via `TCTEST_TOKEN_GH`
- teamcity token is now read from `TCTEST_TOKEN_TC` and shorthand option `-t`
- shorthand opts have been removed from `queue-timeout` and `run-timeout`
- multiple PRs can now be specified `tctest pr 1111,2222,3333`
- results command can now look up a PR ([#30](https://github.com/katbyte/tctest/issues/30))
- support more then 1000 files and files larger then 1MB ([#42](https://github.com/katbyte/tctest/issues/42))
- build queue can now be skipped ([#52](https://github.com/katbyte/tctest/issues/52))
- pr and teamcity build can now be opened in a browser with `--open` flag ([#54](https://github.com/katbyte/tctest/issues/54))

## v0.4.0 (2020-04-12)

- support TeamCity token Auth ([#28](https://github.com/katbyte/tctest/issues/26))
- prevent test auto-detection running all tests unexpectedly ([#28](https://github.com/katbyte/tctest/issues/28))

## v0.3.1 (2020-04-12)

- Update TeamCity API endpoints to `/app/rest/2018.1` ([#21](https://github.com/katbyte/tctest/issues/21))

## v0.3.0 (2020-02-29)

- Prevent `_test_test.go` PR file lookup ([#20](https://github.com/katbyte/tctest/issues/20))
- Rename `status` to `results` for accuracy ([#15](https://github.com/katbyte/tctest/issues/15))
- Rename root command to `branch` to avoid triggering accidental builds on typos ([#15](https://github.com/katbyte/tctest/issues/15))
- Add `--wait` option to `pr`, `branch`, and `results` commands ([#15](https://github.com/katbyte/tctest/issues/15))
- `list` command no longer triggers a build when no tests are found ([#15](https://github.com/katbyte/tctest/issues/15))
- Usage information is no longer displayed after non-usage-related errors ([#15](https://github.com/katbyte/tctest/issues/15))
- `results` command will display a warning if an in-progress build would give incomplete results ([#15](https://github.com/katbyte/tctest/issues/15))
- `results` will now inform the user if the specified build is still queued ([#15](https://github.com/katbyte/tctest/issues/15))

## v0.2.0 (2020-01-22)

- support `azurerm` new package per service structure
- new command `status` to get tests results ([#5](https://github.com/katbyte/tctest/issues/5))

## v0.1.0 (2019-05-24)

Initial release!
