package imapclient_test

import (
	"bytes"
	"crypto/tls"
	"net"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/cogentcore/go-imap/v2"
	"github.com/cogentcore/go-imap/v2/imapclient"
	"github.com/cogentcore/go-imap/v2/imapserver"
	"github.com/cogentcore/go-imap/v2/imapserver/imapmemserver"
)

// gmailMessage is a message in gmailSession's single mailbox.
type gmailMessage struct {
	uid    imap.UID
	flags  []imap.Flag
	labels []string
	msgID  uint64
	thrID  uint64
	modSeq uint64
}

// gmailBackend is a test backend with a single mailbox, which supports the
// Gmail extension and CONDSTORE.
type gmailBackend struct {
	mutex         sync.Mutex
	msgs          []*gmailMessage
	highestModSeq uint64

	// Last received arguments, for tests to inspect
	criteria     *imap.SearchCriteria
	storeOptions *imap.StoreOptions
	selectOpts   *imap.SelectOptions
}

func newGmailBackend() *gmailBackend {
	return &gmailBackend{
		highestModSeq: 12,
		msgs: []*gmailMessage{
			{uid: 1, labels: []string{"\\Inbox", "\\Important"}, msgID: 1278455344230334865, thrID: 1266894439832287888, modSeq: 10},
			{uid: 2, flags: []imap.Flag{imap.FlagSeen}, labels: []string{"Work stuff", "Réunions"}, msgID: 1278455344230334866, thrID: 1266894439832287888, modSeq: 11},
			{uid: 3, labels: []string{}, msgID: 1278455344230334867, thrID: 1266894439832287999, modSeq: 12},
		},
	}
}

// gmailSession is an imapserver.SessionGmail. Authentication is delegated to
// an imapmemserver session.
type gmailSession struct {
	imapserver.Session
	backend *gmailBackend
}

var _ imapserver.SessionGmail = (*gmailSession)(nil)

func (sess *gmailSession) Select(mailbox string, options *imap.SelectOptions) (*imap.SelectData, error) {
	b := sess.backend
	b.mutex.Lock()
	defer b.mutex.Unlock()
	b.selectOpts = options
	return &imap.SelectData{
		Flags:          []imap.Flag{imap.FlagSeen, imap.FlagFlagged},
		PermanentFlags: []imap.Flag{imap.FlagSeen, imap.FlagFlagged},
		NumMessages:    uint32(len(b.msgs)),
		UIDNext:        imap.UID(len(b.msgs) + 1),
		UIDValidity:    1,
		HighestModSeq:  b.highestModSeq,
	}, nil
}

func (sess *gmailSession) Unselect() error {
	return nil
}

func (sess *gmailSession) Poll(w *imapserver.UpdateWriter, allowExpunge bool) error {
	return nil
}

func (sess *gmailSession) Status(mailbox string, options *imap.StatusOptions) (*imap.StatusData, error) {
	b := sess.backend
	b.mutex.Lock()
	defer b.mutex.Unlock()
	num := uint32(len(b.msgs))
	return &imap.StatusData{
		Mailbox:       mailbox,
		NumMessages:   &num,
		HighestModSeq: b.highestModSeq,
	}, nil
}

func (b *gmailBackend) forEach(numSet imap.NumSet, f func(seqNum uint32, msg *gmailMessage)) {
	for i, msg := range b.msgs {
		seqNum := uint32(i + 1)
		var ok bool
		switch numSet := numSet.(type) {
		case imap.SeqSet:
			ok = numSet.Contains(seqNum)
		case imap.UIDSet:
			ok = numSet.Contains(msg.uid)
		}
		if ok {
			f(seqNum, msg)
		}
	}
}

func (sess *gmailSession) Fetch(w *imapserver.FetchWriter, numSet imap.NumSet, options *imap.FetchOptions) error {
	b := sess.backend
	b.mutex.Lock()
	defer b.mutex.Unlock()
	var err error
	b.forEach(numSet, func(seqNum uint32, msg *gmailMessage) {
		if err != nil || (options.ChangedSince != 0 && msg.modSeq <= options.ChangedSince) {
			return
		}
		respWriter := w.CreateMessage(seqNum)
		if options.UID {
			respWriter.WriteUID(msg.uid)
		}
		if options.Flags {
			respWriter.WriteFlags(msg.flags)
		}
		if options.ModSeq {
			respWriter.WriteModSeq(msg.modSeq)
		}
		if options.GmailMsgID {
			respWriter.WriteGmailMsgID(msg.msgID)
		}
		if options.GmailThreadID {
			respWriter.WriteGmailThreadID(msg.thrID)
		}
		if options.GmailLabels {
			respWriter.WriteGmailLabels(msg.labels)
		}
		err = respWriter.Close()
	})
	return err
}

func (sess *gmailSession) Store(w *imapserver.FetchWriter, numSet imap.NumSet, flags *imap.StoreFlags, options *imap.StoreOptions) error {
	b := sess.backend
	b.mutex.Lock()
	defer b.mutex.Unlock()
	b.storeOptions = options
	_, isUID := numSet.(imap.UIDSet)
	var err error
	b.forEach(numSet, func(seqNum uint32, msg *gmailMessage) {
		if err != nil || (options.UnchangedSince != 0 && msg.modSeq > options.UnchangedSince) {
			return
		}
		switch flags.Op {
		case imap.StoreFlagsSet:
			msg.flags = flags.Flags
		case imap.StoreFlagsAdd:
			msg.flags = append(msg.flags, flags.Flags...)
		}
		b.highestModSeq++
		msg.modSeq = b.highestModSeq
		respWriter := w.CreateMessage(seqNum)
		if isUID {
			respWriter.WriteUID(msg.uid)
		}
		respWriter.WriteFlags(msg.flags)
		respWriter.WriteModSeq(msg.modSeq)
		err = respWriter.Close()
	})
	return err
}

func (sess *gmailSession) StoreGmailLabels(w *imapserver.FetchWriter, numSet imap.NumSet, store *imap.StoreGmailLabels) error {
	b := sess.backend
	b.mutex.Lock()
	defer b.mutex.Unlock()
	_, isUID := numSet.(imap.UIDSet)
	var err error
	b.forEach(numSet, func(seqNum uint32, msg *gmailMessage) {
		if err != nil {
			return
		}
		switch store.Op {
		case imap.StoreFlagsSet:
			msg.labels = append([]string{}, store.Labels...)
		case imap.StoreFlagsAdd:
			for _, label := range store.Labels {
				if !containsString(msg.labels, label) {
					msg.labels = append(msg.labels, label)
				}
			}
		case imap.StoreFlagsDel:
			var labels []string
			for _, label := range msg.labels {
				if !containsString(store.Labels, label) {
					labels = append(labels, label)
				}
			}
			msg.labels = labels
		}
		b.highestModSeq++
		msg.modSeq = b.highestModSeq
		if store.Silent {
			return
		}
		respWriter := w.CreateMessage(seqNum)
		if isUID {
			respWriter.WriteUID(msg.uid)
		}
		respWriter.WriteGmailLabels(msg.labels)
		err = respWriter.Close()
	})
	return err
}

func (sess *gmailSession) Search(kind imapserver.NumKind, criteria *imap.SearchCriteria, options *imap.SearchOptions) (*imap.SearchData, error) {
	b := sess.backend
	b.mutex.Lock()
	defer b.mutex.Unlock()
	b.criteria = criteria
	var seqSet imap.SeqSet
	var uidSet imap.UIDSet
	for i, msg := range b.msgs {
		// X-GM-RAW queries are not evaluated, only recorded
		if criteria.GmailThreadID != 0 && msg.thrID != criteria.GmailThreadID {
			continue
		}
		seqSet.AddNum(uint32(i + 1))
		uidSet.AddNum(msg.uid)
	}
	data := &imap.SearchData{All: seqSet}
	if kind == imapserver.NumKindUID {
		data.All = uidSet
	}
	return data, nil
}

func containsString(l []string, s string) bool {
	for _, v := range l {
		if v == s {
			return true
		}
	}
	return false
}

// newGmailClientServerPair starts a server backed by a gmailBackend and
// returns a selected client connected to it, along with the client's
// transcript.
func newGmailClientServerPair(t *testing.T, caps imap.CapSet) (*imapclient.Client, *gmailBackend, *transcript, func()) {
	memServer := imapmemserver.New()
	memServer.AddUser(imapmemserver.NewUser(testUsername, testPassword))
	backend := newGmailBackend()

	cert, err := tls.X509KeyPair([]byte(rsaCertPEM), []byte(rsaKeyPEM))
	if err != nil {
		t.Fatalf("tls.X509KeyPair() = %v", err)
	}

	server := imapserver.New(&imapserver.Options{
		NewSession: func(conn *imapserver.Conn) (imapserver.Session, *imapserver.GreetingData, error) {
			return &gmailSession{Session: memServer.NewSession(), backend: backend}, nil, nil
		},
		TLSConfig:    &tls.Config{Certificates: []tls.Certificate{cert}},
		InsecureAuth: true,
		Caps:         caps,
	})

	ln, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		t.Fatalf("net.Listen() = %v", err)
	}
	go server.Serve(ln)

	conn, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatalf("net.Dial() = %v", err)
	}

	tr := &transcript{}
	client := imapclient.New(conn, &imapclient.Options{DebugWriter: tr})
	if err := client.Login(testUsername, testPassword).Wait(); err != nil {
		t.Fatalf("Login().Wait() = %v", err)
	}
	if !client.Caps().Has(imap.CapGmailExt1) {
		t.Errorf("server does not advertise %v", imap.CapGmailExt1)
	}
	return client, backend, tr, func() {
		client.Close()
		server.Close()
	}
}

// transcript records the client's debug output.
type transcript struct {
	mutex sync.Mutex
	buf   bytes.Buffer
}

func (tr *transcript) Write(b []byte) (int, error) {
	tr.mutex.Lock()
	defer tr.mutex.Unlock()
	return tr.buf.Write(b)
}

func (tr *transcript) String() string {
	tr.mutex.Lock()
	defer tr.mutex.Unlock()
	return tr.buf.String()
}

var gmailCaps = imap.CapSet{
	imap.CapIMAP4rev1: {},
	imap.CapCondStore: {},
	imap.CapGmailExt1: {},
	imap.CapESearch:   {},
}

func TestGmailFetch(t *testing.T) {
	for _, utf8Accept := range []bool{false, true} {
		name := "utf7"
		if utf8Accept {
			name = "utf8"
		}
		t.Run(name, func(t *testing.T) {
			client, _, tr, closeFunc := newGmailClientServerPair(t, gmailCaps)
			defer closeFunc()

			if utf8Accept {
				if _, err := client.Enable(imap.CapUTF8Accept).Wait(); err != nil {
					t.Fatalf("Enable() = %v", err)
				}
			}
			if _, err := client.Select("[Gmail]/All Mail", nil).Wait(); err != nil {
				t.Fatalf("Select() = %v", err)
			}

			msgs, err := client.Fetch(imap.UIDSetNum(1, 2, 3), &imap.FetchOptions{
				GmailMsgID:    true,
				GmailThreadID: true,
				GmailLabels:   true,
			}).Collect()
			if err != nil {
				t.Fatalf("Fetch().Collect() = %v", err)
			} else if len(msgs) != 3 {
				t.Fatalf("len(msgs) = %v, want 3", len(msgs))
			}

			want := []struct {
				uid          imap.UID
				msgID, thrID uint64
				labels       []string
			}{
				{1, 1278455344230334865, 1266894439832287888, []string{"\\Inbox", "\\Important"}},
				{2, 1278455344230334866, 1266894439832287888, []string{"Work stuff", "Réunions"}},
				{3, 1278455344230334867, 1266894439832287999, []string{}},
			}
			for i, msg := range msgs {
				w := want[i]
				if msg.UID != w.uid || msg.GmailMsgID != w.msgID || msg.GmailThreadID != w.thrID {
					t.Errorf("msg %v: UID = %v, GmailMsgID = %v, GmailThreadID = %v, want %v, %v, %v",
						i, msg.UID, msg.GmailMsgID, msg.GmailThreadID, w.uid, w.msgID, w.thrID)
				}
				if !reflect.DeepEqual(msg.GmailLabels, w.labels) {
					t.Errorf("msg %v: GmailLabels = %q, want %q", i, msg.GmailLabels, w.labels)
				}
			}

			s := tr.String()
			if !strings.Contains(s, "X-GM-LABELS (\\Inbox \\Important)") {
				t.Errorf("system labels not sent as atoms:\n%v", s)
			}
			wantLabel := `"R&AOk-unions"`
			if utf8Accept {
				wantLabel = `"Réunions"`
			}
			if !strings.Contains(s, wantLabel) {
				t.Errorf("transcript does not contain %v:\n%v", wantLabel, s)
			}
		})
	}
}

func TestGmailStoreLabels(t *testing.T) {
	client, backend, tr, closeFunc := newGmailClientServerPair(t, gmailCaps)
	defer closeFunc()

	if _, err := client.Select("INBOX", nil).Wait(); err != nil {
		t.Fatalf("Select() = %v", err)
	}

	msgs, err := client.StoreGmailLabels(imap.SeqSetNum(3), &imap.StoreGmailLabels{
		Op:     imap.StoreFlagsAdd,
		Labels: []string{"\\Starred", "Work stuff", "Réunions"},
	}).Collect()
	if err != nil {
		t.Fatalf("StoreGmailLabels(+).Collect() = %v", err)
	} else if len(msgs) != 1 {
		t.Fatalf("len(msgs) = %v, want 1", len(msgs))
	}
	want := []string{"\\Starred", "Work stuff", "Réunions"}
	if !reflect.DeepEqual(msgs[0].GmailLabels, want) {
		t.Errorf("GmailLabels = %q, want %q", msgs[0].GmailLabels, want)
	}
	if !reflect.DeepEqual(backend.msgs[2].labels, want) {
		t.Errorf("backend labels = %q, want %q", backend.msgs[2].labels, want)
	}
	if s := tr.String(); !strings.Contains(s, `STORE 3 +X-GM-LABELS (\Starred "Work stuff" "R&AOk-unions")`) {
		t.Errorf("unexpected STORE command:\n%v", s)
	}

	// UID STORE, removing labels
	msgs, err = client.StoreGmailLabels(imap.UIDSetNum(3), &imap.StoreGmailLabels{
		Op:     imap.StoreFlagsDel,
		Labels: []string{"\\Starred", "Réunions"},
	}).Collect()
	if err != nil {
		t.Fatalf("StoreGmailLabels(-).Collect() = %v", err)
	} else if len(msgs) != 1 {
		t.Fatalf("len(msgs) = %v, want 1", len(msgs))
	}
	want = []string{"Work stuff"}
	if msgs[0].UID != 3 || !reflect.DeepEqual(msgs[0].GmailLabels, want) {
		t.Errorf("UID = %v, GmailLabels = %q, want 3, %q", msgs[0].UID, msgs[0].GmailLabels, want)
	}

	// Silent replace
	msgs, err = client.StoreGmailLabels(imap.SeqSetNum(3), &imap.StoreGmailLabels{
		Op:     imap.StoreFlagsSet,
		Silent: true,
		Labels: []string{"\\Inbox"},
	}).Collect()
	if err != nil {
		t.Fatalf("StoreGmailLabels(silent).Collect() = %v", err)
	} else if len(msgs) != 0 {
		t.Errorf("len(msgs) = %v, want 0", len(msgs))
	}
	if want := []string{"\\Inbox"}; !reflect.DeepEqual(backend.msgs[2].labels, want) {
		t.Errorf("backend labels = %q, want %q", backend.msgs[2].labels, want)
	}
}

func TestGmailStoreLabelsUnsupported(t *testing.T) {
	// imapmemserver doesn't implement imapserver.SessionGmail
	client, server := newClientServerPair(t, imap.ConnStateSelected)
	defer client.Close()
	defer server.Close()

	err := client.StoreGmailLabels(imap.SeqSetNum(1), &imap.StoreGmailLabels{
		Op:     imap.StoreFlagsAdd,
		Labels: []string{"foo"},
	}).Close()
	if imapErr, ok := err.(*imap.Error); !ok || imapErr.Type != imap.StatusResponseTypeBad {
		t.Errorf("StoreGmailLabels() = %v, want BAD", err)
	}
}

func TestGmailSearch(t *testing.T) {
	client, backend, tr, closeFunc := newGmailClientServerPair(t, gmailCaps)
	defer closeFunc()

	if _, err := client.Select("INBOX", nil).Wait(); err != nil {
		t.Fatalf("Select() = %v", err)
	}

	criteria := &imap.SearchCriteria{
		GmailRaw:      `has:attachment in:unread "quoted words"`,
		GmailThreadID: 1266894439832287888,
	}
	data, err := client.UIDSearch(criteria, nil).Wait()
	if err != nil {
		t.Fatalf("UIDSearch() = %v", err)
	}
	if uids := data.AllUIDs(); !reflect.DeepEqual(uids, []imap.UID{1, 2}) {
		t.Errorf("AllUIDs() = %v, want [1 2]", uids)
	}
	if backend.criteria.GmailRaw != criteria.GmailRaw || backend.criteria.GmailThreadID != criteria.GmailThreadID {
		t.Errorf("server got GmailRaw = %q, GmailThreadID = %v, want %q, %v",
			backend.criteria.GmailRaw, backend.criteria.GmailThreadID, criteria.GmailRaw, criteria.GmailThreadID)
	}
	if s := tr.String(); !strings.Contains(s, `UID SEARCH X-GM-RAW "has:attachment in:unread \"quoted words\"" X-GM-THRID 1266894439832287888`) {
		t.Errorf("unexpected SEARCH command:\n%v", s)
	}

	// Non-ASCII query, in a NOT
	criteria = &imap.SearchCriteria{Not: []imap.SearchCriteria{{GmailRaw: "label:réunions"}}}
	if _, err := client.Search(criteria, nil).Wait(); err != nil {
		t.Fatalf("Search() = %v", err)
	}
	if len(backend.criteria.Not) != 1 || backend.criteria.Not[0].GmailRaw != "label:réunions" {
		t.Errorf("server got criteria %+v", backend.criteria)
	}
}

func TestCondStore(t *testing.T) {
	client, backend, tr, closeFunc := newGmailClientServerPair(t, gmailCaps)
	defer closeFunc()

	if !client.Caps().Has(imap.CapCondStore) {
		t.Errorf("server does not advertise %v", imap.CapCondStore)
	}

	statusData, err := client.Status("INBOX", &imap.StatusOptions{NumMessages: true, HighestModSeq: true}).Wait()
	if err != nil {
		t.Fatalf("Status() = %v", err)
	} else if statusData.HighestModSeq != 12 {
		t.Errorf("StatusData.HighestModSeq = %v, want 12", statusData.HighestModSeq)
	}

	selectData, err := client.Select("INBOX", &imap.SelectOptions{CondStore: true}).Wait()
	if err != nil {
		t.Fatalf("Select() = %v", err)
	}
	if !backend.selectOpts.CondStore {
		t.Errorf("SelectOptions.CondStore = false, want true")
	}
	if selectData.HighestModSeq != 12 {
		t.Errorf("SelectData.HighestModSeq = %v, want 12", selectData.HighestModSeq)
	}

	// CHANGEDSINCE implies MODSEQ
	msgs, err := client.Fetch(imap.UIDSetNum(1, 2, 3), &imap.FetchOptions{
		Flags:        true,
		ChangedSince: 10,
	}).Collect()
	if err != nil {
		t.Fatalf("Fetch().Collect() = %v", err)
	} else if len(msgs) != 2 {
		t.Fatalf("len(msgs) = %v, want 2", len(msgs))
	}
	for i, msg := range msgs {
		wantUID, wantModSeq := imap.UID(i+2), uint64(i+11)
		if msg.UID != wantUID || msg.ModSeq != wantModSeq {
			t.Errorf("msg %v: UID = %v, ModSeq = %v, want %v, %v", i, msg.UID, msg.ModSeq, wantUID, wantModSeq)
		}
	}
	if s := tr.String(); !strings.Contains(s, "(CHANGEDSINCE 10)") || !strings.Contains(s, "MODSEQ (11)") {
		t.Errorf("unexpected FETCH exchange:\n%v", s)
	}

	// Explicit MODSEQ
	msgs, err = client.Fetch(imap.SeqSetNum(1), &imap.FetchOptions{ModSeq: true}).Collect()
	if err != nil {
		t.Fatalf("Fetch().Collect() = %v", err)
	} else if len(msgs) != 1 || msgs[0].ModSeq != 10 {
		t.Errorf("msgs = %+v, want ModSeq 10", msgs)
	}

	// Conditional STORE
	msgs, err = client.Store(imap.SeqSetNum(1, 2), &imap.StoreFlags{
		Op:    imap.StoreFlagsAdd,
		Flags: []imap.Flag{imap.FlagFlagged},
	}, &imap.StoreOptions{UnchangedSince: 10}).Collect()
	if err != nil {
		t.Fatalf("Store().Collect() = %v", err)
	}
	if backend.storeOptions.UnchangedSince != 10 {
		t.Errorf("StoreOptions.UnchangedSince = %v, want 10", backend.storeOptions.UnchangedSince)
	}
	if len(msgs) != 1 || msgs[0].SeqNum != 1 || msgs[0].ModSeq != 13 {
		t.Errorf("msgs = %+v, want message 1 with ModSeq 13", msgs)
	}
}
