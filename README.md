# tctest

A command-line utility to trigger builds in teamcity to run provider acceptance tests. Given a PR# it can find the files modified, tests to run and generate a TEST_PATTERN.    

Example:
![pr-example](_docs/example.png)

basic help:
![help](_docs/help.png)


## Configuration

While all commands can be configured from the command line, environment variables can be used instead. By creating a file such as [`set_env_example.sh`](_docs/set_env_example.sh), it can then be sourced:
![env](_docs/env.png) 

## Basic Usage

To run a build on a branch with a test pattern:
```bash
tctest master TestAcc -s ci.katbyte.me -b AzureRm -u katbyte
```
or when environment variables are set
```bash
tctest master TestAcc
```

## For a PR

To run a build on the merge branch with a specific test pattern:
```bash
tctest pr 3232 TestAcc -s ci.katbyte.me -b AzureRm -u katbyte -r terraform-providers/terraform-provider-azurerm
```


if no test pattern is specified the modified files in the PR will be checked and it will be generated automatically
```bash
tctest pr 3232
```  

or simply list all the tests discovered
```bash
tctest pr list 3232
```