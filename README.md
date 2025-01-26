# git-protocol-v2 [![Go Reference](https://pkg.go.dev/badge/github.com/bored-engineer/git-protocol-v2.svg)](https://pkg.go.dev/github.com/bored-engineer/git-protocol-v2)
A Golang package for reading/writing the [Git protocol-v2 format](https://git-scm.com/docs/protocol-v2).

## Usage
The [cmd/](./cmd/) directory contains example CLIs which demonstrate a working [protocol-v2](https://git-scm.com/docs/protocol-v2) client using the HTTP transport, ex:

```shell
$ go install github.com/bored-engineer/git-protocol-v2/...@latest
$ git-v2-capabilities https://github.com/bored-engineer/git-protocol-v2
agent=git/github-8e2ff7c5586f
ls-refs=unborn
fetch=shallow wait-for-done filter
server-option
object-format=sha1
$ git-v2-ls-refs --symrefs https://github.com/bored-engineer/git-protocol-v2
b0819254e1af48969fa88aff09e7563cc5fcec6d HEAD symref-target:refs/heads/main
b0819254e1af48969fa88aff09e7563cc5fcec6d refs/heads/main
$ git-v2-fetch --want b0819254e1af48969fa88aff09e7563cc5fcec6d https://github.com/bored-engineer/git-protocol-v2 > fetch.pack
Enumerating objects: 43, done.
Counting objects: 100% (43/43), done.
Compressing objects: 100% (30/30), done.
Total 43 (delta 13), reused 30 (delta 7), pack-reused 0 (from 0)
$ file git-protocol-v2.pack
git-protocol-v2.pack: Git pack, version 2, 43 objects
```

