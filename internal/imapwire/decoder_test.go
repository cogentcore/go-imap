package imapwire_test

import (
	"bufio"
	"errors"
	"strings"
	"testing"

	"github.com/cogentcore/go-imap/v2"
	"github.com/cogentcore/go-imap/v2/internal/imapwire"
)

func newTestDecoder(s string) *imapwire.Decoder {
	return imapwire.NewDecoder(bufio.NewReader(strings.NewReader(s)), imapwire.ConnSideServer)
}

func expectDecoderExpectError(t *testing.T, dec *imapwire.Decoder) {
	t.Helper()
	err := dec.Err()
	if err == nil {
		t.Fatal("decoder has no error")
	}
	var decErr *imapwire.DecoderExpectError
	if !errors.As(err, &decErr) {
		t.Fatalf("decoder error is %T (%v), want *DecoderExpectError", err, err)
	}
}

func TestExpectNumSetInvalid(t *testing.T) {
	dec := newTestDecoder("abc\r\n")

	var numSet imap.NumSet
	if dec.ExpectNumSet(imapwire.NumKindSeq, &numSet) {
		t.Fatal("ExpectNumSet accepted an invalid sequence-set")
	}
	expectDecoderExpectError(t, dec)
}

func TestExpectMailboxInvalidUTF7(t *testing.T) {
	dec := newTestDecoder("&\r\n")

	var name string
	if dec.ExpectMailbox(&name) {
		t.Fatal("ExpectMailbox accepted a name that is not modified UTF-7")
	}
	expectDecoderExpectError(t, dec)
}

func TestExpectGmailLabel(t *testing.T) {
	dec := newTestDecoder(`\Inbox "\\Important" Work "Work stuff" "R&AOk-unions" {4}` + "\r\n" + `a"b)` + "\r\n")
	want := []string{"\\Inbox", "\\Important", "Work", "Work stuff", "Réunions", `a"b)`}
	for i, w := range want {
		if i > 0 && !dec.ExpectSP() {
			t.Fatalf("ExpectSP() = %v", dec.Err())
		}
		var label string
		if !dec.ExpectGmailLabel(&label) {
			t.Fatalf("ExpectGmailLabel() = %v", dec.Err())
		} else if label != w {
			t.Errorf("ExpectGmailLabel() = %q, want %q", label, w)
		}
	}

	dec = newTestDecoder("\"&\"\r\n")
	var label string
	if dec.ExpectGmailLabel(&label) {
		t.Fatal("ExpectGmailLabel accepted an invalid label")
	}
	expectDecoderExpectError(t, dec)
}
