# go-kit logging compatibility module

This repository-owned workspace module exists only because `github.com/aliyun/aliyun-log-go-sdk v0.1.127` imports the historical package paths `github.com/go-kit/kit/log` and `github.com/go-kit/kit/log/level`.

The module delegates that narrow API surface to `github.com/go-kit/log v0.2.1`. It intentionally does not carry the monolithic `github.com/go-kit/kit` dependency graph and must not become a general application dependency. First-party code imports `github.com/go-kit/log` directly.

Removal condition: retire or replace the direct Aliyun SLS SDK adapter, or adopt an upstream SDK version that no longer imports the historical package path.

Root `go.work` keeps this module in the workspace. The affected product modules also carry an exact version-scoped local replacement for `github.com/go-kit/kit v0.10.0` because `go mod tidy` resolves each module independently. Every replacement must target this directory; external or unversioned substitutions are forbidden.
