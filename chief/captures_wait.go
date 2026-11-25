package chief

import (
	"context"
	"crypto/tls"
	"errors"
	"net/http"
	"net/url"
	"slices"
	"time"
)

const (
	// recordingPoll and idlePoll are how often a wait asks for changes, while
	// something in its scope is being recorded and while nothing is.
	recordingPoll = 2 * time.Second
	idlePoll      = 10 * time.Second
	// changesQuiet is how long no new change has to come before a wait returns
	// those it has, so that a sentence still being spoken arrives whole.
	changesQuiet = 5 * time.Second
	// pollTimeout bounds one request, so that a connection lost without a word, as
	// when a laptop sleeps, doesn't stall a wait that has no deadline.
	pollTimeout = 30 * time.Second
)

// WaitOptions is what Captures.Wait waits for. Cursor is a CaptureResponse's
// ChangesCursor or the Cursor of changes already read; without one the wait is
// for what happens from now on. ProjectID and CaptureID narrow it unless they
// are empty. UntilStopped waits for recording to end instead of for a pause.
type WaitOptions struct {
	Cursor       string
	ProjectID    string
	CaptureID    string
	UntilStopped bool
}

// Wait blocks until the captures in scope change, and returns the changes since
// the cursor once no new one has come for five seconds. With UntilStopped it
// returns once nothing in scope is being recorded and the changes are quiet; if
// nothing was being recorded to begin with that is at once, changes or not.
//
// It also returns, without an error, when ctx ends, with the changes it has by
// then: none of them is an empty page. With UntilStopped it returns none while
// something is still being recorded: Held counts the entries it kept back, and
// the Cursor is the one the wait began from, so waiting again from it brings
// them once recording stops. Recording is nil when not even one request was
// answered, and then says nothing of what is being recorded.
//
// The returned Cursor covers exactly the returned changes. Waiting again from it
// skips nothing, however the wait ended. What came in several pages is merged:
// an entry that changed more than once, or that was filed in another capture, is
// there once, as it was last, in the place it first came in.
//
// Who said a spoken entry is a guess that can change for as long as its capture
// is being recorded, which UntilStopped waits out.
//
// Network errors, 429s and 5xx are retried until ctx ends, so a wait without a
// deadline outlives an outage. Any other error is returned and loses nothing,
// since the caller's cursor still stands; a 410 (see IsGone) means the cursor is
// too old, and the captures have to be read again.
func (s *CapturesService) Wait(ctx context.Context, opts WaitOptions) (*CaptureChangesResponse, error) {
	w := captureWait{captures: s, opts: opts, start: opts.Cursor, page: CaptureChangesResponse{Cursor: opts.Cursor}}
	for {
		err := w.drain(ctx)
		switch {
		case ctx.Err() != nil:
			return w.result(), nil
		case err != nil && !passing(err):
			return nil, err
		case err == nil && w.done():
			return w.result(), nil
		}
		if s.client.sleep(ctx, w.interval()) != nil {
			return w.result(), nil
		}
	}
}

// captureWait is one Wait under way. page's cursor is past every change merged
// so far and no other.
type captureWait struct {
	captures *CapturesService
	opts     WaitOptions
	page     CaptureChangesResponse
	merged   mergedChanges
	// start is the cursor the wait began from: the caller's, or what the first
	// answer said now is.
	start string
	// began is what the first answer said of recording.
	began recordingStart
	// changedAt is when the last change came, zero when none has.
	changedAt time.Time
}

type recordingStart int

const (
	recordingUnknown recordingStart = iota
	recordingOff
	recordingOn
)

// drain reads changes until there are no more. One that fails keeps the pages
// before it.
func (w *captureWait) drain(ctx context.Context) error {
	opts := []ListOption{WithLimit(100)}
	if w.opts.ProjectID != "" {
		opts = append(opts, WithProjectFilter(w.opts.ProjectID))
	}
	if w.opts.CaptureID != "" {
		opts = append(opts, WithCaptureFilter(w.opts.CaptureID))
	}
	for {
		page, err := w.poll(ctx, opts)
		if err != nil {
			return err
		}
		if len(page.Data) > 0 {
			w.merged.add(page.Data)
			w.changedAt = w.captures.client.now()
		}
		w.page.Cursor, w.page.HasMore = page.Cursor, page.HasMore
		w.page.Recording = append([]string{}, page.Recording...)
		if w.start == "" {
			w.start = page.Cursor
		}
		if w.began == recordingUnknown {
			w.began = recordingOff
			if len(page.Recording) > 0 {
				w.began = recordingOn
			}
		}
		if !page.HasMore {
			return nil
		}
	}
}

func (w *captureWait) poll(ctx context.Context, opts []ListOption) (*CaptureChangesResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, pollTimeout)
	defer cancel()
	return w.captures.Changes(ctx, w.page.Cursor, opts...)
}

// done reports whether the wait is over, after a drain that reached the end.
func (w *captureWait) done() bool {
	if !w.opts.UntilStopped {
		return !w.changedAt.IsZero() && w.quietIn() <= 0
	}
	return len(w.page.Recording) == 0 && (w.began == recordingOff || w.quietIn() <= 0)
}

// quietIn is how long until the changes are quiet if no new one comes.
func (w *captureWait) quietIn() time.Duration {
	if w.changedAt.IsZero() {
		return 0
	}
	return changesQuiet - w.captures.client.now().Sub(w.changedAt)
}

// interval is how long to the next poll: the usual time, or sooner when the
// changes would be quiet and that would end the wait.
func (w *captureWait) interval() time.Duration {
	if len(w.page.Recording) > 0 && w.opts.UntilStopped {
		return recordingPoll
	}
	usual := idlePoll
	if len(w.page.Recording) > 0 {
		usual = recordingPoll
	}
	if quiet := w.quietIn(); quiet > 0 {
		return min(usual, quiet)
	}
	return usual
}

func (w *captureWait) result() *CaptureChangesResponse {
	w.page.Data = w.merged.changes()
	if w.opts.UntilStopped && (len(w.page.Recording) > 0 || w.page.HasMore) {
		for _, change := range w.page.Data {
			w.page.Held += len(change.Entries)
		}
		w.page.HasMore = w.page.HasMore || len(w.page.Data) > 0
		w.page.Data, w.page.Cursor = []CaptureChange{}, w.start
	}
	return &w.page
}

// mergedChanges is changes that came in several pages as one.
type mergedChanges struct {
	order    []string
	captures map[string]*CaptureChange
	// filed is the capture each entry was last seen in.
	filed map[string]string
}

func (m *mergedChanges) add(changes []CaptureChange) {
	if m.captures == nil {
		m.captures, m.filed = map[string]*CaptureChange{}, map[string]string{}
	}
	for _, change := range changes {
		capture, seen := m.captures[change.CaptureID]
		if !seen {
			capture = &CaptureChange{Entries: []CaptureEntryChange{}}
			m.captures[change.CaptureID] = capture
			m.order = append(m.order, change.CaptureID)
		}
		entries := capture.Entries
		if change.Deleted {
			for _, e := range entries {
				delete(m.filed, e.EntryID)
			}
			entries = []CaptureEntryChange{}
		}
		*capture = change
		capture.Entries = entries
		for _, e := range change.Entries {
			m.addEntry(change.CaptureID, e)
		}
	}
}

func (m *mergedChanges) addEntry(captureID string, entry CaptureEntryChange) {
	same := func(e CaptureEntryChange) bool { return e.EntryID == entry.EntryID }
	capture := m.captures[captureID]
	filed, seen := m.filed[entry.EntryID]
	m.filed[entry.EntryID] = captureID
	if seen && filed == captureID {
		capture.Entries[slices.IndexFunc(capture.Entries, same)] = entry
		return
	}
	if seen {
		m.captures[filed].Entries = slices.DeleteFunc(m.captures[filed].Entries, same)
	}
	capture.Entries = append(capture.Entries, entry)
}

func (m *mergedChanges) changes() []CaptureChange {
	out := make([]CaptureChange, 0, len(m.order))
	for _, id := range m.order {
		out = append(out, *m.captures[id])
	}
	return out
}

// passing reports whether a request that failed with err may succeed later as
// it is: the API is overloaded, failing or out of reach.
func passing(err error) bool {
	if apiErr, ok := IsAPIError(err); ok {
		return apiErr.StatusCode == http.StatusTooManyRequests || apiErr.StatusCode >= http.StatusInternalServerError
	}
	if urlErr, ok := errors.AsType[*url.Error](err); ok {
		// A base URL or a certificate that is wrong stays wrong.
		_, untrusted := errors.AsType[*tls.CertificateVerificationError](err)
		u, err := url.Parse(urlErr.URL)
		return err == nil && (u.Scheme == "http" || u.Scheme == "https") && !untrusted
	}
	return errors.Is(err, errReadBody)
}

// sleepContext sleeps for d, or until ctx ends, whose error it then returns.
func sleepContext(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
