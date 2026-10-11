package handlers

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"docklite-agent/internal/testhelpers"
)

func inTempDir(t *testing.T) {
	t.Helper()
	old, _ := os.Getwd()
	testhelpers.AssertNoError(t, os.Chdir(t.TempDir()))
	t.Cleanup(func() { _ = os.Chdir(old) })
}

func TestClassifyDNS(t *testing.T) {
	here := map[string]bool{"203.0.113.7": true}
	cases := []struct {
		ips  []string
		want string
	}{
		{nil, dnsNone},
		{[]string{"203.0.113.7"}, dnsHere},
		{[]string{"198.51.100.9", "203.0.113.7"}, dnsHere},
		{[]string{"104.21.5.5", "172.67.1.1"}, dnsCloudflare}, // proxied through Cloudflare
		{[]string{"198.51.100.9"}, dnsElsewhere},
		{[]string{"104.21.5.5", "198.51.100.9"}, dnsElsewhere}, // mixed: not all Cloudflare
	}
	for _, c := range cases {
		testhelpers.AssertEqual(t, classifyDNS(c.ips, here), c.want)
	}
}

func TestBackoffs(t *testing.T) {
	d, ok := failureBackoff(1)
	testhelpers.AssertTrue(t, ok && d == 5*time.Minute, "first failure retries in 5 minutes")
	_, ok = failureBackoff(4)
	testhelpers.AssertFalse(t, ok, "stops after the third retry")
	testhelpers.AssertTrue(t, dnsBackoff(10*time.Second) < dnsBackoff(3*time.Hour), "DNS checks slow down over time")
}

func TestShortReasonIsActionable(t *testing.T) {
	testhelpers.AssertTrue(t, strings.Contains(shortReason("too many failed authorizations recently"), "too many attempts"), "rate limit")
	testhelpers.AssertTrue(t, strings.Contains(shortReason("Timeout during connect"), "port 80"), "firewall hint")
	testhelpers.AssertTrue(t, strings.Contains(shortReason("Invalid response from http://x/.well-known: 404"), "DNS"), "DNS hint")
}

type fakeCert struct {
	exists   bool
	dns      map[string][]string
	issued   []string
	issueErr error
	issueOut string
}

func (f *fakeCert) deps(now time.Time) certDeps {
	return certDeps{
		now:    func() time.Time { return now },
		exists: func(string) bool { return f.exists },
		lookup: func(d string) []string { return f.dns[d] },
		here:   func() map[string]bool { return map[string]bool{"203.0.113.7": true} },
		issue: func(d string, www bool) (string, error) {
			f.issued = append(f.issued, d+map[bool]string{true: "+www", false: ""}[www])
			return f.issueOut, f.issueErr
		},
	}
}

func newJob(t *testing.T, domain string) (*certQueue, certJob, time.Time) {
	inTempDir(t)
	q := newCertQueue("auto-certs.json")
	now := time.Now()
	q.enqueue(domain, true, 0)
	j, ok := q.next(now.Add(time.Second))
	testhelpers.AssertTrue(t, ok, "job is due")
	return q, j, now
}

func state(q *certQueue, d string) certJob { q.mu.Lock(); defer q.mu.Unlock(); return *q.jobs[d] }

func TestWaitsForDNSWithoutAskingLetsEncrypt(t *testing.T) {
	q, j, now := newJob(t, "new.example.com")
	f := &fakeCert{dns: map[string][]string{"new.example.com": nil}}
	q.processCertJob(j, f.deps(now))
	testhelpers.AssertEqual(t, len(f.issued), 0) // never asks while DNS isn't ready
	testhelpers.AssertEqual(t, state(q, "new.example.com").State, certWaiting)
	testhelpers.AssertTrue(t, strings.Contains(state(q, "new.example.com").Message, "Waiting for DNS"), "says what it's waiting for")

	f.dns["new.example.com"] = []string{"198.51.100.9"} // points to another server
	q.processCertJob(state(q, "new.example.com"), f.deps(now))
	testhelpers.AssertEqual(t, len(f.issued), 0)
	testhelpers.AssertTrue(t, strings.Contains(state(q, "new.example.com").Message, "different server"), "says it points elsewhere")
}

func TestIssuesOnceDNSPointsHereAndIncludesWww(t *testing.T) {
	q, j, now := newJob(t, "shop.example.com")
	f := &fakeCert{dns: map[string][]string{"shop.example.com": {"203.0.113.7"}, "www.shop.example.com": {"203.0.113.7"}}}
	q.processCertJob(j, f.deps(now))
	testhelpers.AssertEqual(t, strings.Join(f.issued, ","), "shop.example.com+www")
	testhelpers.AssertEqual(t, state(q, "shop.example.com").State, certIssued)
}

func TestLeavesOutWwwThatDoesNotPointHere(t *testing.T) {
	q, j, now := newJob(t, "shop.example.com")
	f := &fakeCert{dns: map[string][]string{"shop.example.com": {"203.0.113.7"}, "www.shop.example.com": nil}}
	q.processCertJob(j, f.deps(now))
	testhelpers.AssertEqual(t, strings.Join(f.issued, ","), "shop.example.com") // one bad name would fail the whole request
	testhelpers.AssertTrue(t, strings.Contains(state(q, "shop.example.com").Message, "not included"), "tells you")
}

func TestCloudflareProxiedDomainIsIssued(t *testing.T) {
	q, j, now := newJob(t, "cf.example.com")
	f := &fakeCert{dns: map[string][]string{"cf.example.com": {"104.21.5.5"}, "www.cf.example.com": {"172.67.1.1"}}}
	q.processCertJob(j, f.deps(now))
	testhelpers.AssertEqual(t, strings.Join(f.issued, ","), "cf.example.com+www")
}

func TestAlreadyHavingACertificateIsNotReissued(t *testing.T) {
	q, j, now := newJob(t, "done.example.com")
	f := &fakeCert{exists: true}
	q.processCertJob(j, f.deps(now))
	testhelpers.AssertEqual(t, len(f.issued), 0)
	testhelpers.AssertEqual(t, state(q, "done.example.com").State, certIssued)
}

func TestFailuresRetryPolitelyThenStop(t *testing.T) {
	q, j, now := newJob(t, "bad.example.com")
	f := &fakeCert{dns: map[string][]string{"bad.example.com": {"203.0.113.7"}, "www.bad.example.com": {"203.0.113.7"}}, issueErr: os.ErrInvalid, issueOut: "Timeout during connect"}
	for i := 1; i <= 3; i++ {
		q.processCertJob(state(q, "bad.example.com"), f.deps(now))
		s := state(q, "bad.example.com")
		testhelpers.AssertEqual(t, s.State, certWaiting)
		testhelpers.AssertEqual(t, s.FailedAttempts, i)
		testhelpers.AssertTrue(t, s.NextAttempt.After(now), "retry is scheduled in the future")
	}
	q.processCertJob(state(q, "bad.example.com"), f.deps(now))
	s := state(q, "bad.example.com")
	testhelpers.AssertEqual(t, s.State, certFailed) // stops, and says what to do
	testhelpers.AssertTrue(t, strings.Contains(s.Message, "Try now"), "tells you how to retry")
	_ = j
}

func TestGivesUpAfterTwoDaysOfWrongDNS(t *testing.T) {
	q, j, now := newJob(t, "never.example.com")
	f := &fakeCert{dns: map[string][]string{}}
	q.processCertJob(j, f.deps(now.Add(49*time.Hour)))
	testhelpers.AssertEqual(t, state(q, "never.example.com").State, certFailed)
}

func TestJobsSurviveARestart(t *testing.T) {
	inTempDir(t)
	q := newCertQueue("auto-certs.json")
	q.enqueue("keep.example.com", true, time.Hour)
	again := newCertQueue("auto-certs.json")
	got := again.snapshot()
	testhelpers.AssertEqual(t, len(got), 1)
	testhelpers.AssertEqual(t, got[0].Domain, "keep.example.com")
	_, err := os.Stat(filepath.Join(".", "auto-certs.json"))
	testhelpers.AssertNoError(t, err)
}
