# go-imap

This is the [Cogent Core](https://cogentcore.org) fork of
[emersion/go-imap](https://github.com/emersion/go-imap), with the Gmail
extension of IMAP (X-GM-EXT-1: X-GM-MSGID, X-GM-THRID, X-GM-LABELS, and
X-GM-RAW) in the client and the server, and CONDSTORE in the server, which
are off unless they are used, or listed in the capabilities of a server.

Its branches:

- `v2` is the same as upstream.
- `gmail` has the changes, as commits on upstream, with its module path.
- `cogentcore` is `gmail` with the module path `github.com/cogentcore/go-imap/v2`,
  for use without a replace directive. Upstream (`v2`) and `gmail` are merged
  into it to update it.

To merge upstream, in a clone of this fork (`origin`):

```sh
git remote add upstream https://github.com/emersion/go-imap.git # once
git fetch upstream origin

# v2: fast-forward to upstream
git checkout v2 && git merge --ff-only upstream/v2 && git push origin v2

# gmail: merge upstream into the changes
git checkout gmail && git merge v2 && go test ./... && git push origin gmail

# cogentcore: merge gmail; conflicts are only the renamed import paths,
# so keep the github.com/cogentcore/go-imap/v2 side of them
git checkout cogentcore && git merge gmail
grep -rl 'github.com/emersion/go-imap/v2' --include='*.go' --include=go.mod . # must be empty
go test ./... && git push origin cogentcore
```

Then, in a module that uses it, update to the new `cogentcore` commit:

```sh
GOWORK=off go get github.com/cogentcore/go-imap/v2@cogentcore
```


[![Go Reference](https://pkg.go.dev/badge/github.com/cogentcore/go-imap/v2.svg)](https://pkg.go.dev/github.com/cogentcore/go-imap/v2)

An [IMAP4rev2] library for Go.

> **Note**
> This is the README for go-imap v2. This new major version is still in
> development. For go-imap v1, see the [v1 branch].

## Usage

To add go-imap to your project, run:

    go get github.com/cogentcore/go-imap/v2

Documentation and examples for the module are available here:

- [Client docs]
- [Server docs]

## Contributing

See [CONTRIBUTING.md] for contribution guidelines.

## License

MIT

[IMAP4rev2]: https://www.rfc-editor.org/rfc/rfc9051.html
[v1 branch]: https://github.com/emersion/go-imap/tree/v1
[Client docs]: https://pkg.go.dev/github.com/cogentcore/go-imap/v2/imapclient
[Server docs]: https://pkg.go.dev/github.com/cogentcore/go-imap/v2/imapserver
[CONTRIBUTING.md]: https://github.com/emersion/.github/blob/main/CONTRIBUTING.md
