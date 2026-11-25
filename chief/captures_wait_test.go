package chief

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"strconv"
	"sync"
	"testing"
	"time"
)

// fakeFeed is the change feed on a clock of its own, which only a wait's sleeps
// move. Its cursor is how many of its changes a reader has seen.
type fakeFeed struct {
	mu      sync.Mutex
	elapsed time.Duration
	changes []timedChange
	// stopped is when recording ends, zero for never recording at all.
	stopped time.Duration
	// limit is how many changes a page covers.
	limit    int
	requests []string
	naps     int
	// before is called ahead of each answer, with how many there were before it,
	// and returns true to answer itself.
	before func(n int, w http.ResponseWriter, r *http.Request) bool
	// slept is called after each sleep, with how many there were before it.
	slept func(n int)
}

type timedChange struct {
	at     time.Duration
	change CaptureChange
}

func (f *fakeFeed) now() time.Time {
	f.mu.Lock()
	defer f.mu.Unlock()
	return time.Unix(0, 0).Add(f.elapsed)
}

func (f *fakeFeed) sleep(ctx context.Context, d time.Duration) error {
	f.mu.Lock()
	f.elapsed += d
	n, slept := f.naps, f.slept
	f.naps++
	f.mu.Unlock()
	if slept != nil {
		slept(n)
	}
	return ctx.Err()
}

func (f *fakeFeed) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	n, before := len(f.requests), f.before
	f.requests = append(f.requests, r.URL.RawQuery)
	f.mu.Unlock()
	if before != nil && before(n, w, r) {
		return
	}
	f.mu.Lock()
	defer f.mu.Unlock()

	page := CaptureChangesResponse{Data: []CaptureChange{}, Recording: []string{}}
	visible := 0
	for _, c := range f.changes {
		if c.at <= f.elapsed {
			visible++
		}
	}
	if f.elapsed < f.stopped {
		page.Recording = append(page.Recording, "capture-a")
	}
	from, end := visible, visible
	if cursor := r.URL.Query().Get("cursor"); cursor != "" {
		from, _ = strconv.Atoi(cursor)
		end = min(from+f.limit, visible)
	}
	for _, c := range f.changes[from:end] {
		page.Data = append(page.Data, c.change)
	}
	page.Cursor, page.HasMore = strconv.Itoa(end), end < visible
	_ = json.NewEncoder(w).Encode(page)
}

// waitClient is a client that waits on f's clock: a copy for a project, as
// callers make them.
func waitClient(t *testing.T, f *fakeFeed) *Client {
	t.Helper()
	if f.limit == 0 {
		f.limit = 100
	}
	c := newTestClient(t, f.ServeHTTP)
	c.now, c.sleep = f.now, f.sleep
	return c.ForProject("project_b")
}

func entryChange(captureID, entryID, text string) CaptureChange {
	return CaptureChange{CaptureID: captureID, Title: "Title of " + captureID, Entries: []CaptureEntryChange{
		{CaptureEntry: CaptureEntry{EntryID: entryID, Kind: CaptureEntryThought, Author: CaptureAuthorUser, Text: text}},
	}}
}

// said is every entry's text by capture, in order.
func said(page *CaptureChangesResponse) []string {
	var out []string
	for _, c := range page.Data {
		for _, e := range c.Entries {
			out = append(out, c.CaptureID+" "+e.EntryID+" "+e.Text)
		}
	}
	return out
}

func wantSaid(t *testing.T, page *CaptureChangesResponse, want ...string) {
	t.Helper()
	if got := said(page); !slices.Equal(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestChangesSendsItsScopeAndDecodesThePage(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.String(); got != "/v1/captures/changes?capture_id=capture-a&cursor=abc&limit=5&project_id=project_a" {
			t.Errorf("requested %s", got)
		}
		_, _ = io.WriteString(w, `{"data":[{"capture_id":"capture-a","project_id":"project_a","title":"Ideas","deleted":false,
			"entries":[{"entry_id":"e1","kind":"thought","author":"user","at":"2026-05-23T09:00:00Z","text":"hi","deleted":true}]}],
			"cursor":"def","has_more":true,"recording":["capture-a"]}`)
	})
	page, err := c.Captures.Changes(t.Context(), "abc",
		WithLimit(5), WithProjectFilter("project_a"), WithCaptureFilter("capture-a"))
	if err != nil {
		t.Fatalf("changes: %v", err)
	}
	if len(page.Data) != 1 || len(page.Data[0].Entries) != 1 {
		t.Fatalf("page = %+v", page)
	}
	e := page.Data[0].Entries[0]
	if page.Cursor != "def" || !page.HasMore || !slices.Equal(page.Recording, []string{"capture-a"}) ||
		page.Data[0].Title != "Ideas" || e.EntryID != "e1" || e.Text != "hi" || !e.Deleted || e.At.IsZero() {
		t.Errorf("page = %+v", page)
	}
}

func TestWaitReturnsOnceChangesAreQuiet(t *testing.T) {
	const sec = time.Second
	for _, tc := range []struct {
		name    string
		stopped time.Duration
		changes []timedChange
		want    []string
		elapsed time.Duration
	}{
		{
			// Polled at 0, 2 and 4, where the last change shows, then 6, 8 and 9.
			name: "recording", stopped: time.Hour,
			changes: []timedChange{
				{at: 1 * sec, change: entryChange("capture-a", "e1", "I")},
				{at: 3 * sec, change: entryChange("capture-a", "e1", "I think")},
			},
			want: []string{"capture-a e1 I think"}, elapsed: 9 * sec,
		},
		{
			// Polled at 0, 10 and 20, where the change shows, then once it is quiet.
			name:    "not recording",
			changes: []timedChange{{at: 12 * sec, change: entryChange("capture-a", "e1", "typed")}},
			want:    []string{"capture-a e1 typed"}, elapsed: 25 * sec,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := &fakeFeed{changes: tc.changes, stopped: tc.stopped}
			page, err := waitClient(t, f).Captures.Wait(t.Context(), WaitOptions{Cursor: "0"})
			if err != nil {
				t.Fatalf("wait: %v", err)
			}
			wantSaid(t, page, tc.want...)
			if f.elapsed != tc.elapsed || page.Cursor != strconv.Itoa(len(tc.changes)) {
				t.Errorf("returned at %s with cursor %s, want %s", f.elapsed, page.Cursor, tc.elapsed)
			}
		})
	}
}

func TestWaitUntilStopped(t *testing.T) {
	const sec = time.Second
	for _, tc := range []struct {
		name    string
		stopped time.Duration
		changes []timedChange
		elapsed time.Duration
	}{
		{name: "nothing recording", elapsed: 0},
		{name: "nothing recording, with changes", changes: []timedChange{
			{at: 0, change: entryChange("capture-a", "e1", "a")},
			{at: 0, change: entryChange("capture-a", "e2", "b")},
			{at: 0, change: entryChange("capture-a", "e3", "c")},
		}, elapsed: 0},
		{name: "stops with no change", stopped: 6 * sec, elapsed: 6 * sec},
		{name: "pauses for long while recording", stopped: 60 * sec, changes: []timedChange{
			{at: 10 * sec, change: entryChange("capture-a", "e1", "a")},
		}, elapsed: 60 * sec},
		{name: "a last change just before it stops", stopped: 60 * sec, changes: []timedChange{
			{at: 10 * sec, change: entryChange("capture-a", "e1", "a")},
			{at: 57 * sec, change: entryChange("capture-a", "e2", "b")},
		}, elapsed: 63 * sec},
		{name: "a change after it stopped", stopped: 60 * sec, changes: []timedChange{
			{at: 57 * sec, change: entryChange("capture-a", "e1", "a")},
			{at: 63 * sec, change: entryChange("capture-a", "e2", "b")},
		}, elapsed: 68 * sec},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := &fakeFeed{changes: tc.changes, stopped: tc.stopped, limit: 2}
			page, err := waitClient(t, f).Captures.Wait(t.Context(), WaitOptions{Cursor: "0", UntilStopped: true})
			if err != nil {
				t.Fatalf("wait: %v", err)
			}
			if got := len(said(page)); got != len(tc.changes) || page.Cursor != strconv.Itoa(len(tc.changes)) || page.HasMore {
				t.Errorf("got %d entries at cursor %s, want %d", got, page.Cursor, len(tc.changes))
			}
			if f.elapsed != tc.elapsed {
				t.Errorf("returned at %s, want %s", f.elapsed, tc.elapsed)
			}
			if page.Recording == nil || len(page.Recording) > 0 {
				t.Errorf("recording = %#v, want an empty list", page.Recording)
			}
		})
	}
}

// TestWaitHoldsWhatIsStillBeingRecorded pins that words whose speaker can still
// change never come out of an until-stopped wait, and that none is lost: the
// next wait, from the cursor the first began from, brings them all.
func TestWaitHoldsWhatIsStillBeingRecorded(t *testing.T) {
	for _, tc := range []struct {
		name   string
		cursor string
		want   []string
	}{
		{name: "from a cursor", cursor: "0", want: []string{"capture-a e0 before", "capture-a e1 one", "capture-a e2 two"}},
		{name: "from now", want: []string{"capture-a e1 one", "capture-a e2 two"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			f := &fakeFeed{stopped: 60 * time.Second, changes: []timedChange{
				{change: entryChange("capture-a", "e0", "before")},
				{at: time.Second, change: entryChange("capture-a", "e1", "on")},
				{at: 3 * time.Second, change: entryChange("capture-a", "e1", "one")},
				{at: 50 * time.Second, change: entryChange("capture-a", "e2", "two")},
			}}
			f.slept = func(n int) {
				if n == 10 {
					cancel()
				}
			}
			c := waitClient(t, f)
			held, err := c.Captures.Wait(ctx, WaitOptions{Cursor: tc.cursor, UntilStopped: true})
			if err != nil {
				t.Fatalf("wait: %v", err)
			}
			wantSaid(t, held)
			if want := len(tc.want) - 1; held.Held != want || len(held.Recording) != 1 || held.Data == nil {
				t.Errorf("held = %+v, want %d entries held", held, want)
			}

			f.slept = nil
			all, err := c.Captures.Wait(t.Context(), WaitOptions{Cursor: held.Cursor, UntilStopped: true})
			if err != nil {
				t.Fatalf("following wait: %v", err)
			}
			wantSaid(t, all, tc.want...)
			if all.Held != 0 || all.HasMore {
				t.Errorf("after it stopped: %+v", all)
			}
		})
	}
}

func TestWaitWithoutACursorStartsFromNow(t *testing.T) {
	f := &fakeFeed{stopped: 4 * time.Second, changes: []timedChange{
		{at: 0, change: entryChange("capture-a", "e1", "before")},
		{at: time.Second, change: entryChange("capture-a", "e2", "after")},
	}}
	page, err := waitClient(t, f).Captures.Wait(t.Context(),
		WaitOptions{ProjectID: "project_a", CaptureID: "capture-a", UntilStopped: true})
	if err != nil {
		t.Fatalf("wait: %v", err)
	}
	wantSaid(t, page, "capture-a e2 after")
	if want := "capture_id=capture-a&limit=100&project_id=project_a"; f.requests[0] != want {
		t.Errorf("first request %q, want %q", f.requests[0], want)
	}
	if want := "capture_id=capture-a&cursor=1&limit=100&project_id=project_a"; f.requests[1] != want {
		t.Errorf("second request %q, want %q", f.requests[1], want)
	}
}

// TestWaitSkipsNothingWhereverItIsCut pins the cursor contract: a wait cut off
// at any request or any sleep, followed by one from the cursor it returned, has
// between the two of them every change, the ones made meanwhile included.
func TestWaitSkipsNothingWhereverItIsCut(t *testing.T) {
	feed := func() *fakeFeed {
		f := &fakeFeed{stopped: 20 * time.Second, limit: 2}
		for i := range 11 {
			f.changes = append(f.changes, timedChange{
				at: time.Duration(i/3) * 5 * time.Second, change: entryChange("capture-a", fmt.Sprintf("e%02d", i), "x"),
			})
		}
		return f
	}
	var want []string
	for _, c := range feed().changes {
		want = append(want, c.change.Entries[0].EntryID)
	}

	for _, cutAt := range []string{"unanswered request", "answered request", "sleep"} {
		for _, untilStopped := range []bool{false, true} {
			for n := range 16 {
				t.Run(fmt.Sprintf("%s %d until stopped %t", cutAt, n, untilStopped), func(t *testing.T) {
					ctx, cancel := context.WithCancel(t.Context())
					defer cancel()
					f := feed()
					switch cutAt {
					case "unanswered request":
						f.before = func(i int, _ http.ResponseWriter, r *http.Request) bool {
							if i == n {
								cancel()
								<-r.Context().Done()
							}
							return i == n
						}
					case "answered request":
						f.before = func(i int, _ http.ResponseWriter, _ *http.Request) bool {
							if i == n {
								cancel()
							}
							return false
						}
					case "sleep":
						f.slept = func(i int) {
							if i == n {
								cancel()
							}
						}
					}
					c := waitClient(t, f)

					first, err := c.Captures.Wait(ctx, WaitOptions{Cursor: "0", UntilStopped: untilStopped})
					if err != nil {
						t.Fatalf("cut wait: %v", err)
					}
					f.mu.Lock()
					f.before, f.slept = nil, nil
					f.elapsed, f.stopped = time.Hour, 0
					f.mu.Unlock()
					rest, err := c.Captures.Wait(t.Context(), WaitOptions{Cursor: first.Cursor, UntilStopped: true})
					if err != nil {
						t.Fatalf("following wait: %v", err)
					}

					var got []string
					for _, page := range []*CaptureChangesResponse{first, rest} {
						for _, c := range page.Data {
							for _, e := range c.Entries {
								got = append(got, e.EntryID)
							}
						}
					}
					if !slices.Equal(got, want) {
						t.Errorf("got %v, then from cursor %s: %v, want every entry once", said(first), first.Cursor, said(rest))
					}
				})
			}
		}
	}
}

func TestWaitThatGetsNothingKeepsItsCursor(t *testing.T) {
	t.Run("nothing came", func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		f := &fakeFeed{stopped: time.Hour, slept: func(n int) {
			if n == 3 {
				cancel()
			}
		}}
		page, err := waitClient(t, f).Captures.Wait(ctx, WaitOptions{Cursor: "0", UntilStopped: true})
		if err != nil || page.Cursor != "0" || page.Data == nil || len(page.Data) > 0 || len(page.Recording) != 1 {
			t.Errorf("got %+v, %v", page, err)
		}
	})
	t.Run("no answer came", func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		page, err := waitClient(t, &fakeFeed{}).Captures.Wait(ctx, WaitOptions{Cursor: "7"})
		if err != nil || page.Cursor != "7" || page.Data == nil || len(page.Data) > 0 || page.Recording != nil {
			t.Errorf("got %+v, %v; want the cursor unchanged and recording unknown", page, err)
		}
	})
}

func TestWaitRetriesWhatPasses(t *testing.T) {
	f := &fakeFeed{changes: []timedChange{{change: entryChange("capture-a", "e1", "a")}}}
	f.before = func(n int, w http.ResponseWriter, _ *http.Request) bool {
		switch n {
		case 0:
			panic(http.ErrAbortHandler)
		case 1:
			w.WriteHeader(http.StatusBadGateway)
		case 2:
			w.WriteHeader(http.StatusTooManyRequests)
		case 3:
			w.Header().Set("Content-Length", "100")
			_, _ = io.WriteString(w, `{"data":`)
		}
		return n < 4
	}
	page, err := waitClient(t, f).Captures.Wait(t.Context(), WaitOptions{Cursor: "0", UntilStopped: true})
	if err != nil {
		t.Fatalf("wait: %v", err)
	}
	wantSaid(t, page, "capture-a e1 a")
	if f.elapsed != 40*time.Second {
		t.Errorf("retried over %s, want one poll every 10s", f.elapsed)
	}
}

func TestWaitReturnsOtherErrors(t *testing.T) {
	for _, status := range []int{http.StatusBadRequest, http.StatusUnauthorized, http.StatusGone} {
		f := &fakeFeed{before: func(_ int, w http.ResponseWriter, _ *http.Request) bool {
			w.WriteHeader(status)
			return true
		}}
		_, err := waitClient(t, f).Captures.Wait(t.Context(), WaitOptions{Cursor: "0"})
		if apiErr, ok := IsAPIError(err); !ok || apiErr.StatusCode != status || f.naps > 0 {
			t.Errorf("status %d: got %v after %d sleeps", status, err, f.naps)
		}
		if got := IsGone(err); got != (status == http.StatusGone) {
			t.Errorf("status %d: IsGone = %t", status, got)
		}
	}
	t.Run("an answer that isn't the feed's", func(t *testing.T) {
		f := &fakeFeed{before: func(_ int, w http.ResponseWriter, _ *http.Request) bool {
			_, _ = io.WriteString(w, "<html>")
			return true
		}}
		if _, err := waitClient(t, f).Captures.Wait(t.Context(), WaitOptions{Cursor: "0"}); err == nil || f.naps > 0 {
			t.Errorf("got %v after %d sleeps", err, f.naps)
		}
	})
	t.Run("an untrusted certificate", func(t *testing.T) {
		server := httptest.NewTLSServer(http.NotFoundHandler())
		defer server.Close()
		c, err := New(WithAPIKey("pat_key"), WithBaseURL(server.URL))
		if err != nil {
			t.Fatalf("new client: %v", err)
		}
		f := &fakeFeed{}
		c.now, c.sleep = f.now, f.sleep
		f.slept = func(int) { t.Fatal("retried") }
		if _, err := c.Captures.Wait(t.Context(), WaitOptions{Cursor: "0"}); err == nil {
			t.Error("got no error")
		}
	})
	t.Run("a base URL that isn't one", func(t *testing.T) {
		for _, baseURL := range []string{"api.example", "://", "http://a b"} {
			c, err := New(WithAPIKey("pat_key"), WithBaseURL(baseURL))
			if err != nil {
				t.Fatalf("new client: %v", err)
			}
			f := &fakeFeed{slept: func(int) { t.Fatalf("retried %q", baseURL) }}
			c.now, c.sleep = f.now, f.sleep
			if _, err := c.Captures.Wait(t.Context(), WaitOptions{Cursor: "0"}); err == nil {
				t.Errorf("%q: got no error", baseURL)
			}
		}
	})
}

// TestWaitMergesPages pins what a reader of the merged page relies on: an entry
// is there once, as it last was and where it now is, in the order first seen,
// and a capture deleted since holds nothing.
func TestWaitMergesPages(t *testing.T) {
	deleted := entryChange("capture-a", "e3", "")
	deleted.Title, deleted.Entries[0].Deleted = "Renamed", true
	f := &fakeFeed{limit: 1, changes: []timedChange{
		{change: entryChange("capture-a", "e1", "one")},
		{change: entryChange("capture-gone", "e9", "nine")},
		{change: entryChange("capture-a", "e2", "two")},
		{change: entryChange("capture-a", "e3", "three")},
		{change: entryChange("capture-a", "e1", "one, longer")},
		// A split filed e2 in another capture.
		{change: entryChange("capture-b", "e2", "two")},
		{change: deleted},
		{change: CaptureChange{CaptureID: "capture-gone", Deleted: true}},
		{change: entryChange("capture-b", "e9", "nine, moved")},
	}}
	page, err := waitClient(t, f).Captures.Wait(t.Context(), WaitOptions{Cursor: "0", UntilStopped: true})
	if err != nil {
		t.Fatalf("wait: %v", err)
	}
	wantSaid(t, page,
		"capture-a e1 one, longer", "capture-a e3 ", "capture-b e2 two", "capture-b e9 nine, moved")
	var captures []string
	for _, c := range page.Data {
		captures = append(captures, fmt.Sprintf("%s %q %t", c.CaptureID, c.Title, c.Deleted))
		if c.Entries == nil {
			t.Errorf("%s has nil entries, which encode as null", c.CaptureID)
		}
	}
	want := []string{`capture-a "Renamed" false`, `capture-gone "" true`, `capture-b "Title of capture-b" false`}
	if !slices.Equal(captures, want) {
		t.Errorf("captures = %q, want %q", captures, want)
	}
	if !page.Data[0].Entries[1].Deleted {
		t.Error("the deleted entry isn't marked deleted")
	}
}
