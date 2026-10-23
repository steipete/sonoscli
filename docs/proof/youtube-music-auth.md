# YouTube Music authentication diagnostic: live CLI proof

Captured 2026-10-20 against a real Sonos speaker and its advertised YouTube Music service, not a mock HTTP endpoint. No playback or volume commands were issued. Only the speaker address in the command is redacted; the error output below is unchanged. No linking code was returned.

## Before: upstream main b7850a276c8c549532f3fcd88c29dad68facf2b4

Built the actual CLI with `go build -o /tmp/sonos-pr29-base-bin ./cmd/sonos` from a detached worktree at the above revision.

```console
$ /tmp/sonos-pr29-base-bin auth smapi begin --service 'YouTube Music' --ip <speaker-ip>
Error: smapi soap http 403 Forbidden
exit: 1
```

## After: PR branch with supported-next-action fix

Built the actual CLI with `go build -o /tmp/sonos-pr29-head ./cmd/sonos` from this revision's production code. The installed CLI was not replaced.

```console
$ /tmp/sonos-pr29-head auth smapi begin --service 'YouTube Music' --ip <speaker-ip>
Error: smapi soap http 403 Forbidden: service requires a registered API client identity (API key or equivalent); account linking alone cannot resolve this; start playback in the Sonos app and control it with sonos, or use sonos play-url with a public URL (playback only, not account authentication)
exit: 1
```

The real service still rejects authentication. The change is the actionable, body-safe error message, not restored account access. Public-URL playback and Sonos-app-started playback are separate from account authentication.

## Base-fail / head-pass regression

Copied this branch's `internal/sonos/smapi_auth_error_test.go` into the detached base worktree (no production changes). Ran on both revisions:

```sh
go test ./internal/sonos -run TestSMAPI_BeginAuthentication_APIClientIdentityError -count=1
```

Base output (exit 1):

```text
--- FAIL: TestSMAPI_BeginAuthentication_APIClientIdentityError (0.00s)
    --- FAIL: TestSMAPI_BeginAuthentication_APIClientIdentityError/unregistered_caller (0.00s)
        smapi_auth_error_test.go:37: API identity hint = false, want true: smapi soap http 403 Forbidden
FAIL
FAIL github.com/steipete/sonoscli/internal/sonos 0.007s
FAIL
```

Head output (exit 0; run with `-v`):

```text
--- PASS: TestSMAPI_BeginAuthentication_APIClientIdentityError (0.00s)
    --- PASS: TestSMAPI_BeginAuthentication_APIClientIdentityError/unregistered_caller (0.00s)
    --- PASS: TestSMAPI_BeginAuthentication_APIClientIdentityError/other_permission_error (0.00s)
    --- PASS: TestSMAPI_BeginAuthentication_APIClientIdentityError/non_JSON_response (0.00s)
PASS
ok github.com/steipete/sonoscli/internal/sonos 0.006s
```

Also passed on the patched branch: `go test ./...`, `go vet ./...`, and `git diff --check`.
