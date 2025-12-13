package sonos

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// curlRoundTripFunc exists for unit tests.
var curlRoundTripFunc = curlRoundTrip

func doRequest(ctx context.Context, httpClient *http.Client, req *http.Request) (*http.Response, error) {
	if req == nil {
		return nil, errors.New("nil request")
	}

	// Ensure we can retry the request body for the curl fallback.
	if req.Body != nil && req.GetBody == nil {
		if err := enableBodyReplay(req); err != nil {
			return nil, err
		}
	}

	resp, err := httpClient.Do(req)
	if err == nil {
		return resp, nil
	}
	if !shouldCurlFallback(req, err) {
		return nil, err
	}

	timeout := fallbackTimeout(ctx, httpClient.Timeout)
	curlResp, curlErr := curlRoundTripFunc(ctx, req, timeout)
	if curlErr != nil {
		// Preserve the original error, but include curl's failure as context.
		return nil, fmt.Errorf("%w (curl fallback failed: %v)", err, curlErr)
	}
	return curlResp, nil
}

func fallbackTimeout(ctx context.Context, clientTimeout time.Duration) time.Duration {
	timeout := clientTimeout
	if dl, ok := ctx.Deadline(); ok {
		remain := time.Until(dl)
		if remain > 0 && (timeout <= 0 || remain < timeout) {
			timeout = remain
		}
	}
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	return timeout
}

func shouldCurlFallback(req *http.Request, err error) bool {
	if req.URL == nil {
		return false
	}
	host := req.URL.Hostname()
	ip := net.ParseIP(host)
	if ip == nil {
		return false
	}
	if !(ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast()) {
		return false
	}
	if req.URL.Scheme != "http" && req.URL.Scheme != "https" {
		return false
	}
	return isTimeoutLike(err)
}

func isTimeoutLike(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return true
	}
	// Go's http client often wraps timeouts as a string like:
	// "context deadline exceeded (Client.Timeout exceeded while awaiting headers)"
	msg := err.Error()
	return strings.Contains(msg, "Client.Timeout exceeded") || strings.Contains(msg, "timeout")
}

func enableBodyReplay(req *http.Request) error {
	const max = 2 << 20 // 2 MiB
	b, err := io.ReadAll(io.LimitReader(req.Body, max+1))
	_ = req.Body.Close()
	if err != nil {
		return err
	}
	if len(b) > max {
		return fmt.Errorf("request body too large for curl fallback: %d bytes", len(b))
	}
	req.GetBody = func() (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(b)), nil
	}
	req.Body = io.NopCloser(bytes.NewReader(b))
	return nil
}

func curlRoundTrip(ctx context.Context, req *http.Request, timeout time.Duration) (*http.Response, error) {
	curlPath, err := exec.LookPath("curl")
	if err != nil {
		return nil, err
	}

	var body []byte
	if req.GetBody != nil {
		rc, err := req.GetBody()
		if err != nil {
			return nil, err
		}
		body, err = io.ReadAll(rc)
		_ = rc.Close()
		if err != nil {
			return nil, err
		}
	}

	args := []string{
		"--silent",
		"--show-error",
		"--include",
		"--max-time", fmt.Sprintf("%.3f", timeout.Seconds()),
		"--connect-timeout", fmt.Sprintf("%.3f", timeout.Seconds()),
		"--request", req.Method,
	}

	// Forward headers (best-effort).
	for k, vals := range req.Header {
		for _, v := range vals {
			args = append(args, "--header", k+": "+v)
		}
	}

	// Avoid 100-continue behavior.
	args = append(args, "--header", "Expect:")

	if len(body) > 0 {
		args = append(args, "--data-binary", "@-")
	}
	args = append(args, req.URL.String())

	cmd := exec.CommandContext(ctx, curlPath, args...)
	if len(body) > 0 {
		cmd.Stdin = bytes.NewReader(body)
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("curl: %w: %s", err, strings.TrimSpace(string(out)))
	}

	resp, err := parseCurlResponse(out, req)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func parseCurlResponse(out []byte, req *http.Request) (*http.Response, error) {
	rest := out
	for {
		header, body, err := splitHeaderBody(rest)
		if err != nil {
			return nil, err
		}
		statusLine, headers, err := parseHeaderBlock(header)
		if err != nil {
			return nil, err
		}

		code, status, proto, major, minor, err := parseStatusLine(statusLine)
		if err != nil {
			return nil, err
		}

		// Skip interim responses (e.g. 100 Continue).
		if code >= 100 && code < 200 && code != 101 {
			rest = body
			continue
		}

		return &http.Response{
			StatusCode:    code,
			Status:        status,
			Proto:         proto,
			ProtoMajor:    major,
			ProtoMinor:    minor,
			Header:        headers,
			Body:          io.NopCloser(bytes.NewReader(body)),
			ContentLength: int64(len(body)),
			Request:       req,
		}, nil
	}
}

func splitHeaderBody(out []byte) (headerText string, body []byte, err error) {
	if i := bytes.Index(out, []byte("\r\n\r\n")); i >= 0 {
		return string(out[:i]), out[i+4:], nil
	}
	if i := bytes.Index(out, []byte("\n\n")); i >= 0 {
		return string(out[:i]), out[i+2:], nil
	}
	return "", nil, errors.New("curl response missing header separator")
}

func parseHeaderBlock(headerText string) (statusLine string, headers http.Header, err error) {
	lines := strings.Split(headerText, "\n")
	if len(lines) == 0 {
		return "", nil, errors.New("empty curl header block")
	}
	statusLine = strings.TrimRight(lines[0], "\r")
	if !strings.HasPrefix(statusLine, "HTTP/") {
		return "", nil, fmt.Errorf("unexpected curl status line: %q", statusLine)
	}

	headers = make(http.Header)
	for _, raw := range lines[1:] {
		line := strings.TrimRight(raw, "\r")
		if line == "" {
			continue
		}
		k, v, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		k = strings.TrimSpace(k)
		v = strings.TrimSpace(v)
		if k == "" {
			continue
		}
		headers.Add(k, v)
	}
	return statusLine, headers, nil
}

func parseStatusLine(statusLine string) (code int, status string, proto string, major int, minor int, err error) {
	parts := strings.SplitN(strings.TrimSpace(statusLine), " ", 3)
	if len(parts) < 2 {
		return 0, "", "", 0, 0, fmt.Errorf("invalid status line: %q", statusLine)
	}
	proto = parts[0]
	code, err = strconv.Atoi(parts[1])
	if err != nil {
		return 0, "", "", 0, 0, fmt.Errorf("invalid status code in status line: %q", statusLine)
	}

	status = parts[1]
	if len(parts) == 3 {
		status += " " + parts[2]
	}

	// Parse "HTTP/1.1" -> (1, 1)
	if strings.HasPrefix(proto, "HTTP/") {
		v := strings.TrimPrefix(proto, "HTTP/")
		if nums := strings.SplitN(v, ".", 2); len(nums) == 2 {
			major, _ = strconv.Atoi(nums[0])
			minor, _ = strconv.Atoi(nums[1])
		}
	}
	return code, status, proto, major, minor, nil
}
