package applemusic

import (
	"fmt"
	"os/exec"
	"runtime"
)

const (
	// AppleMusicWebURL is the URL for Apple Music web client
	AppleMusicWebURL = "https://music.apple.com"

	// AppleMusicBetaURL is the beta web client (sometimes has newer features)
	AppleMusicBetaURL = "https://beta.music.apple.com"
)

// OpenBrowser opens a web browser to the specified URL.
// On macOS, it explicitly uses Safari to avoid opening the Apple Music app.
func OpenBrowser(url string) error {
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "darwin":
		// Use Safari explicitly to avoid macOS opening the Apple Music app
		// for music.apple.com URLs
		cmd = exec.Command("open", "-a", "Safari", url)
	case "linux":
		cmd = exec.Command("xdg-open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		return fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}

	return cmd.Start()
}

// TokenExtractionInstructions returns instructions for manually extracting
// the Apple Music token from the browser.
func TokenExtractionInstructions() string {
	return `To extract your Apple Music token:

1. Sign in to Apple Music at https://music.apple.com
2. Open browser Developer Tools (F12 or Cmd+Option+I)
3. Go to the "Application" tab (Chrome) or "Storage" tab (Firefox)
4. Find "Local Storage" → "https://music.apple.com"
5. Look for a key containing "music.user.token" or similar
6. Copy the token value

Alternatively, in the Console tab, run:
   localStorage.getItem('music.ampwebplay.media-user-token')

The token is a long string starting with something like "Aw..." or similar.`
}

// AuthFlowResult contains the result of an authentication flow.
type AuthFlowResult struct {
	Token        Token
	Instructions string
	ManualEntry  bool // true if user needs to manually enter token
}

// StartAuthFlow initiates the Apple Music authentication flow.
// It opens the browser and returns instructions for token extraction.
func StartAuthFlow() (*AuthFlowResult, error) {
	if err := OpenBrowser(AppleMusicWebURL); err != nil {
		return &AuthFlowResult{
			Instructions: fmt.Sprintf("Could not open browser automatically.\nPlease visit: %s\n\n%s",
				AppleMusicWebURL, TokenExtractionInstructions()),
			ManualEntry: true,
		}, nil
	}

	return &AuthFlowResult{
		Instructions: TokenExtractionInstructions(),
		ManualEntry:  true,
	}, nil
}
