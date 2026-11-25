package imapserver

import (
	"strings"

	"github.com/emersion/go-imap/v2"
	"github.com/emersion/go-imap/v2/internal"
	"github.com/emersion/go-imap/v2/internal/imapwire"
)

func (c *Conn) handleStore(dec *imapwire.Decoder, numKind NumKind) error {
	var (
		numSet  imap.NumSet
		item    string
		options imap.StoreOptions
	)
	if !dec.ExpectSP() || !dec.ExpectNumSet(numKind.wire(), &numSet) || !dec.ExpectSP() {
		return dec.Err()
	}
	isList, err := dec.List(func() error {
		return readStoreModifier(dec, &options)
	})
	if err != nil {
		return err
	} else if isList && !dec.ExpectSP() {
		return dec.Err()
	}
	if !dec.ExpectAtom(&item) || !dec.ExpectSP() {
		return dec.Err()
	}

	item = strings.ToUpper(item)
	silent := strings.HasSuffix(item, ".SILENT")
	item = strings.TrimSuffix(item, ".SILENT")

	var op imap.StoreFlagsOp
	switch {
	case strings.HasPrefix(item, "+"):
		op = imap.StoreFlagsAdd
		item = strings.TrimPrefix(item, "+")
	case strings.HasPrefix(item, "-"):
		op = imap.StoreFlagsDel
		item = strings.TrimPrefix(item, "-")
	default:
		op = imap.StoreFlagsSet
	}

	switch item {
	case "FLAGS":
		flags, err := readStoreValues(dec, func() (imap.Flag, error) {
			return internal.ExpectFlag(dec)
		})
		if err != nil {
			return err
		}
		if !dec.ExpectCRLF() {
			return dec.Err()
		}

		if options.UnchangedSince != 0 && !c.server.options.caps().Has(imap.CapCondStore) {
			return newClientBugError("Unknown STORE modifier")
		}
		if err := c.checkState(imap.ConnStateSelected); err != nil {
			return err
		}

		if options.UnchangedSince != 0 {
			// STORE with UNCHANGEDSINCE is a CONDSTORE-enabling command
			c.mutex.Lock()
			c.enabled[imap.CapCondStore] = struct{}{}
			c.mutex.Unlock()
		}

		w := &FetchWriter{conn: c}
		return c.session.Store(w, numSet, &imap.StoreFlags{
			Op:     op,
			Silent: silent,
			Flags:  flags,
		}, &options)
	case "X-GM-LABELS":
		labels, err := readStoreValues(dec, func() (string, error) {
			var label string
			if !dec.ExpectGmailLabel(&label) {
				return "", dec.Err()
			}
			return label, nil
		})
		if err != nil {
			return err
		}
		if !dec.ExpectCRLF() {
			return dec.Err()
		}

		gmailSession, ok := c.session.(SessionGmail)
		if !ok || !c.server.options.caps().Has(imap.CapGmailExt1) {
			return newClientBugError("STORE can only change FLAGS")
		}
		if options.UnchangedSince != 0 {
			return newClientBugError("UNCHANGEDSINCE is not supported with X-GM-LABELS")
		}
		if err := c.checkState(imap.ConnStateSelected); err != nil {
			return err
		}

		w := &FetchWriter{conn: c}
		return gmailSession.StoreGmailLabels(w, numSet, &imap.StoreGmailLabels{
			Op:     op,
			Silent: silent,
			Labels: labels,
		})
	default:
		return newClientBugError("STORE can only change FLAGS")
	}
}

// readStoreValues reads either a parenthesized list or a space-separated
// sequence of values.
func readStoreValues[T any](dec *imapwire.Decoder, readValue func() (T, error)) ([]T, error) {
	var values []T
	isList, err := dec.List(func() error {
		v, err := readValue()
		if err != nil {
			return err
		}
		values = append(values, v)
		return nil
	})
	if err != nil {
		return nil, err
	} else if !isList {
		for {
			v, err := readValue()
			if err != nil {
				return nil, err
			}
			values = append(values, v)

			if !dec.SP() {
				break
			}
		}
	}
	return values, nil
}

func readStoreModifier(dec *imapwire.Decoder, options *imap.StoreOptions) error {
	var name string
	if !dec.ExpectAtom(&name) {
		return dec.Err()
	}
	switch strings.ToUpper(name) {
	case "UNCHANGEDSINCE":
		if !dec.ExpectSP() || !dec.ExpectModSeq(&options.UnchangedSince) {
			return dec.Err()
		}
	default:
		return newClientBugError("Unknown STORE modifier")
	}
	return nil
}
