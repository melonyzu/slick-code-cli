package transport

import (
	"context"
	"io"
	"net/http"
	"strconv"
	"time"
)

// DefaultAttempts is the total number of tries DoWithRetry makes,
// including the first.
const DefaultAttempts = 3

// baseBackoff is the delay before the first retry; it doubles per
// attempt, capped at maxBackoff, unless the response's Retry-After
// header requests otherwise.
const (
	baseBackoff = 500 * time.Millisecond
	maxBackoff  = 8 * time.Second
)

// DoWithRetry issues the request produced by newRequest through client,
// retrying transient failures: network errors, 429, and 5xx responses.
// newRequest is called once per attempt so each try gets a fresh body.
// The response body of a failed attempt is drained and closed; the
// returned response's body is the caller's to close. Waits respect both
// ctx and the server's Retry-After header.
//
// Retryability is judged by status only — deciding whether a 429 is a
// rate limit or a quota error is the caller's translation concern.
func DoWithRetry(ctx context.Context, client *http.Client, newRequest func() (*http.Request, error)) (*http.Response, error) {
	var (
		resp *http.Response
		err  error
	)

	for attempt := 0; attempt < DefaultAttempts; attempt++ {
		if attempt > 0 {
			if werr := wait(ctx, backoff(attempt, resp)); werr != nil {
				break
			}
		}

		var req *http.Request
		req, err = newRequest()
		if err != nil {
			return nil, err
		}

		resp, err = client.Do(req.WithContext(ctx))
		if err != nil {
			if ctx.Err() != nil {
				return nil, err
			}
			continue
		}
		if !retryable(resp.StatusCode) {
			return resp, nil
		}

		// Keep the final retryable response for the caller to
		// translate; drain earlier ones so connections are reused.
		if attempt < DefaultAttempts-1 {
			drain(resp)
		}
	}

	return resp, err
}

// retryable reports whether a status code indicates a transient failure.
func retryable(status int) bool {
	return status == http.StatusTooManyRequests || status >= 500
}

// backoff returns how long to wait before the given attempt, honoring
// the previous response's Retry-After header when present.
func backoff(attempt int, prev *http.Response) time.Duration {
	if prev != nil {
		if s := prev.Header.Get("Retry-After"); s != "" {
			if secs, err := strconv.Atoi(s); err == nil && secs >= 0 {
				return min(time.Duration(secs)*time.Second, maxBackoff)
			}
		}
	}
	return min(baseBackoff<<(attempt-1), maxBackoff)
}

// wait sleeps for d or until ctx is done, returning ctx's error in the
// latter case.
func wait(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

// drain discards and closes a response body so the underlying
// connection can be reused.
func drain(resp *http.Response) {
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<20))
	resp.Body.Close()
}
