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
