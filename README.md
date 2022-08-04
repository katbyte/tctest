# tctest

![build](https://github.com/katbyte/tctest/actions/workflows/build.yaml/badge.svg)
![lint](https://github.com/katbyte/tctest/actions/workflows/lint.yaml/badge.svg)
[![Go Report Card](https://goreportcard.com/badge/github.com/katbyte/tctest)](https://goreportcard.com/report/github.com/katbyte/tctest)

A command-line utility to trigger builds in teamcity to run provider acceptance tests. Given a PR# it can find the files modified, tests to run and generate a TEST_PATTERN.    

Example:
![pr-example](_docs/example.png)

basic help:
![help](_docs/help.png)

## Installation

To install `tctest` from the command line, you can run:
```bash
go install github.com/katbyte/tctest
```

## Configuration

While all commands can be configured from the command line, environment variables can be used instead. By creating a file such as [`set_env_example.sh`](_docs/set_env_example.sh), it can then be sourced:
![env](_docs/env.png) 

## Basic Usage

To run a build on a branch with a test pattern:
```bash
tctest branch master TestAcc -s ci.katbyte.me -b AzureRm -u katbyte
```
or when environment variables are set:
```bash
tctest branch master TestAcc
```

## For a PR

To run a build on the merge branch with a specific test pattern:
```bash
tctest pr 3232 TestAcc -s ci.katbyte.me -b AzureRm -u katbyte -r terraform-providers/terraform-provider-azurerm
```


If no test pattern is specified the modified files in the PR will be checked and it will be generated automatically:
```bash
tctest pr 3232
```  

To list all the tests discovered for a given PR:
```bash
tctest list 3232
```

To run tests against a PR and display results when complete:
```bash
tctest pr 3232 --wait
```

## Build results: 
*By TeamCity Build Number*

To show the PASS/FAIL/SKIP results for a TeamCity build number:
```bash
tctest results 12345
```

To wait for a running or queued build to complete and then show the results:
```bash
tctest results 12345 --wait
```

*By Github PR Number*

To show the PASS/FAIL/SKIP results for **all** TeamCity builds for a Github PR:
```bash
tctest results pr 12345
```
To show the PASS/FAIL/SKIP results for the **latest** TeamCity build for a Github PR:
```bash
tctest results pr 12345 --latest
```
To wait for a running or queued build to complete and then show the results:
```bash
tctest results pr 12345 --wait
```
