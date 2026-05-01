# tctest

[![GitHub release](https://img.shields.io/github/v/release/katbyte/tctest?color=blueviolet)](https://github.com/katbyte/tctest/releases/latest)
![build](https://github.com/katbyte/tctest/actions/workflows/build.yaml/badge.svg)
![lint](https://github.com/katbyte/tctest/actions/workflows/lint.yaml/badge.svg)
[![Go Report Card](https://goreportcard.com/badge/github.com/katbyte/tctest)](https://goreportcard.com/report/github.com/katbyte/tctest)
[![License](https://img.shields.io/github/license/katbyte/tctest?color=blue)](https://github.com/katbyte/tctest/blob/main/LICENSE)

A command-line utility to trigger builds in TeamCity to run provider acceptance tests. Given a PR number it can find the files modified, discover the tests to run, and generate a `TEST_PATTERN` automatically.    


## Installation

```bash
go install github.com/katbyte/tctest@latest
```

## Configuration

All options can be passed as command-line flags but most can also be set via environment variables. Create a file like [`set_env_example.sh`](.github/images/set_env_example.sh) and source it.


### Environment Variables

| Variable | Flag | Description |
|---|---|---|
| `TCTEST_SERVER` | `--server`, `-s` | TeamCity server URL |
| `TCTEST_BUILDTYPEID` | `--buildtypeid`, `-b` | TeamCity build configuration ID |
| `TCTEST_TOKEN_TC` | `--token-tc`, `-t` | TeamCity authentication token |
| `TCTEST_USER` | `--username` | TeamCity username (alternative to token) |
| `TCTEST_PASS` | `--password` | TeamCity password (alternative to token) |
| `TCTEST_PROPERTIES` | `--properties`, `-p` | Default build parameters in `KEY=VALUE;KEY2=VALUE2` format |
| `GITHUB_TOKEN` | `--token-gh` | GitHub OAuth token |
| `TCTEST_REPO` | `--repo`, `-r` | GitHub repository (e.g. `hashicorp/terraform-provider-azurerm`) |
| `TCTEST_FILEREGEX` | `--fileregex` | Regex to filter PR files for test discovery |
| `TCTEST_SPLIT_TESTS_ON` | `--splitteston` | Character to split test names on (default: `_`) |
| `TCTEST_WAIT` | `--wait`, `-w` | Wait for builds to complete |
| `TCTEST_LATESTBUILD` | `--latest` | Get the latest build |
| `TCTEST_SKIP_QUEUE` | `--skip-queue`, `-q` | Put the build to the top of the queue |
| `TCTEST_OPEN_BROWSER` | `--open`, `-o` | Open PR and build URLs in the browser |
| `TCTEST_BUILD_TAGS` | `--tag` | Build tags to add to triggered builds |
| `TCTEST_COMMENT` | `--comment`, `-c` | Post a GitHub comment with test results |
| `TCTEST_FORCE_OLD_UI` | `--build-link-force-old-ui` | Force build URLs to use the classic TeamCity UI |
| `TCTEST_OUTPUT_QUIET` | `--quiet` | Minimal machine-readable output |
| `TCTEST_OUTPUT_JSON` | `--json` | Output build results as a JSON array |
| `TCTEST_OUTPUT_SILENT` | `--silent` | Suppress all output |

## Commands

### `branch` — Run tests on a branch

Triggers acceptance tests matching a regex for a specific branch.

```bash
# with flags
tctest branch master TestAcc -s ci.katbyte.me -b AzureRm

# with environment variables set
tctest branch master TestAcc

# alias
tctest b master TestAcc
```

### `pr` — Run tests for a PR

Discovers tests from modified PR files and triggers builds. If no test regex is specified, it automatically determines which tests to run based on the changed files.

```bash
# auto-discover tests from PR files
tctest pr 3232

# specify a test pattern manually
tctest pr 3232 TestAccAzureRMVirtualNetwork

# multiple PRs at once
tctest pr 3232,5454,7676

# wait for builds to complete and show results
tctest pr 3232 --wait

# open PR and build in browser
tctest pr 3232 --open
```

#### Service targeting with `--service`

Use `--service` to target specific service(s). When used without `--all`, it still discovers tests from PR files but only triggers builds for the specified services. With `--all`, it runs `TestAcc` (all tests) for those services.

```bash
# discover tests from PR, but only run for the network service
tctest pr 3232 --service network

# discover tests from PR for multiple services
tctest pr 3232 --service network,compute

# run ALL tests for a specific service (no test discovery)
tctest pr 3232 --service network --all

# run ALL tests for ALL services (no test discovery)
tctest pr 3232 --service all --all

# invalid service names will error with a list of valid services
tctest pr 3232 --service fakesvc
# ERROR: invalid service(s): fakesvc
# valid services: aadb2c, advisor, apimanagement, ...
```

#### Run all discovered tests with `--all`

Without `--service`, `--all` overrides the discovered test regex with `TestAcc` to run all tests for the affected services:

```bash
tctest pr 3232 --all
```

#### Post a GitHub comment with `--comment` / `-c`

Adds `POST_GITHUB_COMMENT=true` to the build properties, telling TeamCity to post test results as a comment on the PR:

```bash
tctest pr 3232 --comment
tctest pr 3232 -c
```

### `prs` — Run tests for multiple PRs with filters

Discovers all open PRs matching specified filters and triggers builds for each.

```bash
# all open PRs by specific authors
tctest prs -a katbyte,author2

# PRs with specific labels (all must match)
tctest prs -l needs-testing,service/network

# PRs with any matching label
tctest prs --f-labels-any needs-testing,ready-for-review

# PRs by author with a specific label
tctest prs -a katbyte -l needs-testing

# PRs not in draft
tctest prs -d

# PRs created within the last 24 hours
tctest prs --f-created-time 24h

# PRs updated within the last 2 hours
tctest prs --f-updated-time 2h

# PRs with a specific milestone
tctest prs -m v3.0.0

# PRs without a specific milestone
tctest prs -m -v3.0.0

# PRs matching a title regex (case-insensitive)
tctest prs --f-title-regex "network.*fix"

# combine filters with a custom test pattern
tctest prs TestAccAzureRM -a katbyte -l needs-testing
```

#### Filter flags

| Flag | Short | Description |
|---|---|---|
| `--f-authors` | `-a` | Only test PRs by these authors (comma-separated) |
| `--f-labels-all` | `-l` | Only test PRs matching **all** label conditions. Prefix with `-` to negate |
| `--f-labels-any` | | Only test PRs matching **any** label condition. Prefix with `-` to negate |
| `--f-milestone` | `-m` | Filter by milestone. Prefix with `-` to exclude |
| `--f-drafts` | `-d` | Filter out draft PRs |
| `--f-created-time` | | Only PRs created within this duration (e.g. `24h`, `7d`) |
| `--f-updated-time` | | Only PRs updated within this duration |
| `--f-title-regex` | | Filter PRs by title using case-insensitive regex |

### `list` — Preview discovered tests

Lists the tests that would be triggered for a PR without actually starting a build.

```bash
tctest list 3232
```

### `results` — Show build results

#### By TeamCity build ID

```bash
# show PASS/FAIL/SKIP results
tctest results 12345

# wait for a running build to complete, then show results
tctest results 12345 --wait
```

#### By GitHub PR number

```bash
# show results for all builds for a PR
tctest results pr 12345

# show results for only the latest build
tctest results pr 12345 --latest

# wait for builds to complete, then show results
tctest results pr 12345 --wait
```

### `version` — Print version

```bash
tctest version
```

## Build Options

These flags apply to any command that triggers a build:

| Flag | Short | Description |
|---|---|---|
| `--properties` | `-p` | Build parameters in `KEY=VALUE;KEY2=VALUE2` format |
| `--comment` | `-c` | Post a GitHub comment with test results (`POST_GITHUB_COMMENT=true`) |
| `--skip-queue` | `-q` | Put the build to the top of the queue |
| `--wait` | `-w` | Wait for the build to complete before exiting |
| `--tag` | | Add tags to the triggered build (comma-separated) |
| `--queue-timeout` | | Minutes to wait for a queued build to start (default: 60) |
| `--run-timeout` | | Minutes to wait for a running build to finish (default: 60) |
| `--open` | `-o` | Open the PR and build URL in the browser |
| `--build-link-force-old-ui` | | Append `&fromSakuraUI=true` to build URLs to force the classic TeamCity UI |

## Output Modes

By default `tctest` prints colorized, verbose output. Use these flags to control output:

| Flag | Description |
|---|---|
| *(default)* | Full colorized output with test discovery details, file listings, and build info |
| `--quiet` | One line per build: `PR@SERVICE@BUILDID URL` |
| `--json` | JSON array of all triggered builds (output at end) |
| `--silent` | Suppress all output (errors still print to stderr) |

### Quiet output

```
32181@costmanagement@658292 https://hashicorp.teamcity.com/viewQueued.html?itemId=658292
32181@mssql@658293 https://hashicorp.teamcity.com/viewQueued.html?itemId=658293
```

### JSON output

```json
[
    {
        "pr": 32181,
        "service": "costmanagement",
        "build_number": 658292,
        "url": "https://hashicorp.teamcity.com/viewQueued.html?itemId=658292"
    },
    {
        "pr": 32181,
        "service": "mssql",
        "build_number": 658293,
        "url": "https://hashicorp.teamcity.com/viewQueued.html?itemId=658293"
    }
]
```

## Test Discovery

When no test regex is provided, `tctest` automatically discovers tests by:

1. Listing all files modified in the PR
2. Filtering to resource, data source, and ephemeral files (configurable via `--fileregex`)
3. Deriving test file names (e.g. `resource_foo.go` → `resource_foo_test.go`)
4. Also discovering related test files (e.g. `resource_foo_list_test.go`, `resource_foo_data_source_test.go`)
5. Downloading test files and extracting test function names using Go AST parsing
6. Grouping tests by service and triggering a separate build per service

Files in `/client/`, `/parse/`, `/validate/` subdirectories and `registration.go`/`resourceids.go` are automatically skipped. Deleted files are also excluded.
