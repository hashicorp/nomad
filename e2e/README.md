End to End Tests
================

This package contains integration tests that are not run by default. To run them use the `-integration` flag. Example:

```
$ cd e2e/rescheduling/
$ go test -integration
Running Suite: Server Side Restart Tests
========================================
Random Seed: 1520633027
Will run 7 of 7 specs

•••••••
Ran 7 of 7 Specs in 4.231 seconds
SUCCESS! -- 7 Passed | 0 Failed | 0 Pending | 0 Skipped PASS
ok  	github.com/hashicorp/nomad/e2e/rescheduling	4.239s

```