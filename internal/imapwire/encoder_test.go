package imapwire_test

import (
	"bufio"
	"strings"
	"testing"

	"github.com/emersion/go-imap/v2/internal/imapwire"
)

func TestEncoderGmailLabel(t *testing.T) {
	for _, tc := range []struct {
		label      string
		quotedUTF8 bool
		want       string
	}{
		{"\\Inbox", false, `\Inbox`},
		{"Work", false, `"Work"`},
		{"Work stuff", false, `"Work stuff"`},
		{"Réunions", false, `"R&AOk-unions"`},
		{"Réunions", true, `"Réunions"`},
		{"A&B", true, `"A&-B"`},
		{"\\Not a flag", false, `"\\Not a flag"`},
	} {
		var sb strings.Builder
		bw := bufio.NewWriter(&sb)
		enc := imapwire.NewEncoder(bw, imapwire.ConnSideServer)
		enc.QuotedUTF8 = tc.quotedUTF8
		enc.GmailLabel(tc.label)
		bw.Flush()
		if got := sb.String(); got != tc.want {
			t.Errorf("GmailLabel(%q) = %v, want %v", tc.label, got, tc.want)
		}
	}
}
