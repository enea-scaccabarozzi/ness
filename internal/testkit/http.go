package testkit

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/rogpeppe/go-internal/testscript"
)

// httpState is http.json: every request made and how many of the next ones fail.
type httpState struct {
	Requests []Recorded `json:"requests"`
	Fail     int        `json:"fail"`
}

// Recorded is one request the fake saw.
type Recorded struct {
	Method string `json:"method"`
	URL    string `json:"url"`
	Body   string `json:"body"`
}

// FakeHTTP stands in for the network: it records every request in Dir/http.json, answers the
// three platforms with increasing ids, answers api.typesafe.ai from Dir/jev/<url path> (first
// line the status, the rest the body; 404 when absent), fails the next Fail requests with 500
// and refuses any other host, so no test can reach a real service (R-QA-024).
type FakeHTTP struct{ Dir string }

func (f FakeHTTP) path() string { return filepath.Join(f.Dir, "http.json") }

// RoundTrip implements http.RoundTripper.
func (f FakeHTTP) RoundTrip(req *http.Request) (*http.Response, error) {
	var st httpState
	if err := loadJSON(f.path(), &st); err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	var body []byte
	if req.Body != nil {
		var err error
		if body, err = io.ReadAll(req.Body); err != nil {
			return nil, err
		}
	}
	st.Requests = append(st.Requests, Recorded{Method: req.Method, URL: req.URL.String(), Body: string(body)})
	failing := st.Fail > 0
	status, reply, err := answer(&st, req.URL.Host)
	if err == nil && req.URL.Host == jevHost && !failing {
		status, reply, err = f.jev(req.URL.Path)
	}
	if err != nil {
		return nil, err
	}
	if serr := saveJSON(f.path(), st); serr != nil {
		return nil, serr
	}
	return &http.Response{
		StatusCode: status, Status: strconv.Itoa(status) + " " + http.StatusText(status),
		Header: http.Header{"Content-Type": []string{"application/json"}},
		Body:   io.NopCloser(bytes.NewReader([]byte(reply))), Request: req,
	}, nil
}

// answer is the reply of host to the request just recorded in st; an unknown host is an error.
func answer(st *httpState, host string) (status int, reply string, err error) {
	n := len(st.Requests)
	switch {
	case host != "slack.com" && host != "api.telegram.org" && host != "api.github.com" && host != jevHost:
		return 0, "", fmt.Errorf("fake http: no route to %s (tests never reach the network)", host)
	case st.Fail > 0:
		st.Fail--
		return http.StatusInternalServerError, `{"ok":false,"error":"fake failure"}`, nil
	case host == "slack.com":
		return http.StatusOK, fmt.Sprintf(`{"ok":true,"ts":"1700000000.%06d"}`, n), nil
	case host == "api.telegram.org":
		return http.StatusOK, fmt.Sprintf(`{"ok":true,"result":{"message_id":%d}}`, n), nil
	}
	return http.StatusCreated, fmt.Sprintf(`{"id":%d}`, n), nil
}

// jevHost is the Jev API's default host, answered from the files under Dir/jev.
const jevHost = "api.typesafe.ai"

// jev is the canned reply at Dir/jev/path: "STATUS\nBODY"; 404 when no file is there.
func (f FakeHTTP) jev(path string) (status int, reply string, err error) {
	b, err := os.ReadFile(filepath.Join(f.Dir, "jev", filepath.Clean("/"+path))) //nolint:gosec // test fixture dir
	if os.IsNotExist(err) {
		return http.StatusNotFound, `{"detail":"Not Found"}`, nil
	}
	if err != nil {
		return 0, "", err
	}
	line, body, _ := strings.Cut(string(b), "\n")
	if status, err = strconv.Atoi(strings.TrimSpace(line)); err != nil {
		return 0, "", fmt.Errorf("fake http: %s: first line must be the status: %w", path, err)
	}
	return status, body, nil
}

// Requests returns what was recorded so far.
func (f FakeHTTP) Requests() ([]Recorded, error) {
	var st httpState
	err := loadJSON(f.path(), &st)
	if os.IsNotExist(err) {
		return nil, nil
	}
	return st.Requests, err
}

// httpCmd is `fake http`:
//
//	fake http requests N    fail unless N requests were made (negatable)
//	fake http body N FILE   write the Nth request's body plus a newline to FILE
//	fake http url N VAR     set $VAR to the Nth request's URL
//	fake http fail N        the next N requests answer 500
func httpCmd(ts *testscript.TestScript, neg bool, dir string, args []string) {
	f := FakeHTTP{Dir: dir}
	if len(args) < 2 {
		ts.Fatalf("usage: fake http requests N | body N FILE | url N VAR | fail N")
	}
	reqs, err := f.Requests()
	ts.Check(err)
	n, err := strconv.Atoi(args[1])
	ts.Check(err)
	switch args[0] {
	case "requests":
		if (len(reqs) == n) == neg {
			ts.Fatalf("%d requests recorded, want %d", len(reqs), n)
		}
	case "body", "url":
		httpPick(ts, neg, args, reqs)
	case "fail":
		var st httpState
		_ = loadJSON(f.path(), &st) //nolint:errcheck // a missing file is an empty state
		st.Fail = n
		ts.Check(saveJSON(f.path(), st))
	default:
		ts.Fatalf("fake http: unknown subcommand %s", args[0])
	}
}

// httpPick is `fake http body N FILE` and `fake http url N VAR`.
func httpPick(ts *testscript.TestScript, neg bool, args []string, reqs []Recorded) {
	n, _ := strconv.Atoi(args[1]) //nolint:errcheck // httpCmd parsed it
	if neg || len(args) != 3 || n < 1 || n > len(reqs) {
		ts.Fatalf("fake http %s N FILE|VAR with 1 <= N <= %d", args[0], len(reqs))
	}
	if args[0] == "url" {
		ts.Setenv(args[2], reqs[n-1].URL)
		return
	}
	ts.Check(os.WriteFile(ts.MkAbs(args[2]), []byte(reqs[n-1].Body+"\n"), 0o600))
}
