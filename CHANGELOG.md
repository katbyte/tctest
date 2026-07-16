## v0.9.0 (2026-07-16)

- bump golang.org/x/crypto from 0.47.0 to 0.52.0 ([#84](https://github.com/katbyte/tctest/pull/84))
- enforce exact args, fix draft flag parsing/filtering, improve error handling for PR tests, and fix TeamCity auth ([#83](https://github.com/katbyte/tctest/pull/83))
- add retry transport for GitHub API calls ([#83](https://github.com/katbyte/tctest/pull/83))
- add `--dry-run` and `--concurrency` flags ([#82](https://github.com/katbyte/tctest/pull/82))
- switch to concurrent file downloads and using raw GitHub URLs vs API ([#81](https://github.com/katbyte/tctest/pull/81))
- refactor test discovery to support AWS provider file layout, use suffix patterns for related test discovery, and skip non-Go files ([#80](https://github.com/katbyte/tctest/pull/80))

## v0.8.0 (2026-06-18)
- add a new flag to reappend the split character ([#79](https://github.com/katbyte/tctest/pull/79))

## v0.7.0 (2026-05-01)

- colour-coded output for changed and derived files ([#74](https://github.com/katbyte/tctest/pull/74))
- fix default `--fileregex` to allow numbers in file names ([#75](https://github.com/katbyte/tctest/pull/75))
- add `--comment`/`-c` option to post GitHub comments on PRs with test results ([#76](https://github.com/katbyte/tctest/pull/76))
- add `--service` flag to target specific services or all services ([#77](https://github.com/katbyte/tctest/pull/77))
- add `--quiet`, `--json`, and `--silent` output modes for machine-readable output ([#77](https://github.com/katbyte/tctest/pull/77))
- add `--build-link-force-old-ui` to force classic TeamCity UI links ([#77](https://github.com/katbyte/tctest/pull/77))
- deprecate `--buildtypeid` in favour of `--build-type-id` with opt-in `--build-type-id-add-service-suffix` ([#77](https://github.com/katbyte/tctest/pull/77))

## v0.6.0 (2026-01-30)

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
