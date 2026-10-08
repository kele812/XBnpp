package watchaccess

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func newTestReporter(t *testing.T) *Reporter {
	t.Helper()
	r, e := New(Config{URL: "https://watch.example.com", Node: strings.Repeat("a", 48), Secret: strings.Repeat("b", 48)})
	if e != nil {
		t.Fatal(e)
	}
	r.policy.Store(&policy{users: map[int]int64{1: 100}, until: time.Now().Add(time.Minute)})
	return r
}
func TestSelectionCapacityAndExpiry(t *testing.T) {
	r := newTestReporter(t)
	r.Record(2, "1.2.3.4", "tcp", "example.com:443")
	if len(r.queue) != 0 {
		t.Fatal("unselected user recorded")
	}
	for i := 0; i < 1010; i++ {
		r.Record(1, "1.2.3.4", "tcp", "tcp:example.com:443")
	}
	if len(r.queue) != 1000 || r.dropped.Load() != 10 {
		t.Fatal("queue not bounded")
	}
	e := <-r.queue
	if e.Host != "example.com" || e.Port != 443 || e.Since != 100 {
		t.Fatal(e)
	}
	r.policy.Store(&policy{users: map[int]int64{1: 100}, until: time.Now().Add(-time.Second)})
	r.Record(1, "1.2.3.4", "tcp", "example.com:443")
	if len(r.queue) != 999 {
		t.Fatal("expired policy allowed collection")
	}
}
func TestIPAndTargetValidation(t *testing.T) {
	r := newTestReporter(t)
	r.Record(1, "::1", "udp", "udp:[2001:db8::1]:53")
	e := <-r.queue
	if e.Host != "2001:db8::1" || e.Port != 53 {
		t.Fatal(e)
	}
	for _, target := range []string{"https://example.com/private", "example.com:0", "bad/path:443"} {
		r.Record(1, "1.2.3.4", "tcp", target)
	}
	if len(r.queue) != 0 || r.dropped.Load() != 3 {
		t.Fatal("invalid targets accepted")
	}
}
func TestSignedPolicyAndUpload(t *testing.T) {
	var uploaded int
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, q *http.Request) {
		b, _ := io.ReadAll(q.Body)
		m := hmac.New(sha256.New, []byte(strings.Repeat("b", 48)))
		m.Write([]byte(q.URL.Path + "\n" + q.Header.Get("X-Watch-Timestamp") + "\n"))
		m.Write(b)
		if q.Header.Get("X-Watch-Signature") != hex.EncodeToString(m.Sum(nil)) {
			t.Error("bad signature")
		}
		if strings.HasSuffix(q.URL.Path, "policy") {
			io.WriteString(w, `{"users":[{"uid":7,"since":123}],"ttlSeconds":90}`)
		} else {
			var payload struct {
				Events []Event `json:"events"`
			}
			json.Unmarshal(b, &payload)
			uploaded = len(payload.Events)
			io.WriteString(w, `{"inserted":1}`)
		}
	}))
	defer srv.Close()
	r := newTestReporter(t)
	r.urls = []string{srv.URL}
	r.client = srv.Client()
	if e := r.syncPolicy(context.Background()); e != nil {
		t.Fatal(e)
	}
	r.Record(1, "1.2.3.4", "tcp", "example.com:443")
	r.Record(7, "1.2.3.4", "tcp", "example.com:443")
	if len(r.queue) != 1 {
		t.Fatal("policy selection ignored")
	}
	e := <-r.queue
	if err := r.send(context.Background(), []Event{e}); err != nil {
		t.Fatal(err)
	}
	if uploaded != 1 {
		t.Fatal(uploaded)
	}
}
func TestConfigRequiresSecureOrigin(t *testing.T) {
	for _, u := range []string{"http://example.com", "https://example.com/path", "https://a:b@example.com", "https://example.com?token=x"} {
		if _, e := New(Config{URL: u, Node: strings.Repeat("a", 48), Secret: strings.Repeat("b", 48)}); e == nil {
			t.Fatal(u)
		}
	}
	r, e := New(Config{})
	if e != nil || r != nil {
		t.Fatal("disabled config")
	}
}

func TestMultipleWatchOriginsFailOverOnlyOnUnavailable(t *testing.T) {
	var backupCalls int
	primaryStatus := http.StatusServiceUnavailable
	primary := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(primaryStatus) }))
	defer primary.Close()
	backup := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		backupCalls++
		io.WriteString(w, `{"users":[],"ttlSeconds":90}`)
	}))
	defer backup.Close()
	r, err := New(Config{URLs: []string{primary.URL, backup.URL}, Node: strings.Repeat("a", 48), Secret: strings.Repeat("b", 48)})
	if err != nil { t.Fatal(err) }
	roots := x509.NewCertPool()
	roots.AddCert(primary.Certificate())
	roots.AddCert(backup.Certificate())
	r.client = &http.Client{Timeout: 8 * time.Second, Transport: &http.Transport{TLSClientConfig: &tls.Config{RootCAs: roots}}}
	if err := r.syncPolicy(context.Background()); err != nil || backupCalls != 1 { t.Fatalf("5xx did not fail over: %v, calls=%d", err, backupCalls) }
	primaryStatus = http.StatusUnauthorized
	if err := r.syncPolicy(context.Background()); err == nil || backupCalls != 1 { t.Fatalf("401 incorrectly failed over: %v, calls=%d", err, backupCalls) }
}

func TestMultipleWatchOriginConfig(t *testing.T) {
	secret := strings.Repeat("b", 48)
	node := strings.Repeat("a", 48)
	r, err := New(Config{URL: "https://one.example.com,https://two.example.com", Node: node, Secret: secret})
	if err != nil || len(r.urls) != 2 { t.Fatalf("comma-separated origins rejected: %v", err) }
	r, err = New(Config{URLs: []string{"https://one.example.com", "https://two.example.com"}, Node: node, Secret: secret})
	if err != nil || len(r.urls) != 2 { t.Fatalf("origin list rejected: %v", err) }
	for _, cfg := range []Config{
		{URLs: []string{"https://one.example.com", "http://unsafe.example.com"}, Node: node, Secret: secret},
		{URL: "https://one.example.com", URLs: []string{"https://two.example.com"}, Node: node, Secret: secret},
	} {
		if _, err := New(cfg); err == nil { t.Fatal("invalid origin list accepted") }
	}
}
