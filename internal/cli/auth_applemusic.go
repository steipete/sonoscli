package cli

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/STop211650/sonoscli/internal/applemusic"
)

func newAuthAppleMusicCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "applemusic",
		Short: "Authenticate with Apple Music",
		Long: `Authenticate with Apple Music for search and playback.

This opens your browser to music.apple.com where you can sign in with your Apple ID.
After signing in, you'll need to extract and paste your authentication token.

The token is stored locally and typically lasts about 6 months.`,
	}

	cmd.AddCommand(newAuthAppleMusicLoginCmd(flags))
	cmd.AddCommand(newAuthAppleMusicStatusCmd(flags))
	cmd.AddCommand(newAuthAppleMusicLogoutCmd(flags))
	return cmd
}

func newAuthAppleMusicLoginCmd(flags *rootFlags) *cobra.Command {
	var developerToken string
	var userToken string
	var storefrontID string

	cmd := &cobra.Command{
		Use:   "login",
		Short: "Sign in to Apple Music",
		Long: `Sign in to Apple Music by opening the browser and extracting your tokens.

Two tokens are required:
  --developer-token: The JWT token (starts with "eyJ...")
  --user-token: The Music User Token (starts with "Av...")

To extract these tokens from the browser:
1. Sign in at https://music.apple.com
2. Open Developer Tools (F12 or Cmd+Option+I)
3. In Console, run: MusicKit.getInstance().developerToken
4. In Console, run: MusicKit.getInstance().musicUserToken
5. Provide both tokens to this command`,
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := applemusic.NewDefaultTokenStore()
			if err != nil {
				return err
			}

			// If both tokens provided directly, save them
			if strings.TrimSpace(developerToken) != "" && strings.TrimSpace(userToken) != "" {
				token := applemusic.Token{
					DeveloperToken: strings.TrimSpace(developerToken),
					MusicUserToken: strings.TrimSpace(userToken),
					StorefrontID:   strings.TrimSpace(storefrontID),
					CreatedAt:      time.Now().UTC(),
				}
				if err := store.Save(token); err != nil {
					return err
				}

				if isJSON(flags) {
					return writeJSON(cmd, map[string]any{
						"status":       "authenticated",
						"storefrontId": token.StorefrontID,
						"expiresAt":    token.ExpiresAt,
						"tokenPath":    store.Path(),
					})
				}

				fmt.Fprintln(cmd.OutOrStdout(), "Apple Music tokens saved successfully!")
				fmt.Fprintf(cmd.OutOrStdout(), "Token expires around: %s\n", token.ExpiresAt.Format(time.RFC3339))
				return nil
			}

			// Check if only one token was provided
			if strings.TrimSpace(developerToken) != "" || strings.TrimSpace(userToken) != "" {
				return fmt.Errorf("both --developer-token and --user-token are required")
			}

			// Start interactive auth flow
			result, err := applemusic.StartAuthFlow()
			if err != nil {
				return err
			}

			fmt.Fprintln(cmd.OutOrStdout(), "Opening Apple Music in your browser...")
			fmt.Fprintln(cmd.OutOrStdout())
			fmt.Fprintln(cmd.OutOrStdout(), result.Instructions)
			fmt.Fprintln(cmd.OutOrStdout())
			fmt.Fprintln(cmd.OutOrStdout(), "After signing in, run in browser console:")
			fmt.Fprintln(cmd.OutOrStdout(), "  MusicKit.getInstance().developerToken")
			fmt.Fprintln(cmd.OutOrStdout(), "  MusicKit.getInstance().musicUserToken")
			fmt.Fprintln(cmd.OutOrStdout())

			reader := bufio.NewReader(os.Stdin)

			fmt.Fprint(cmd.OutOrStdout(), "Paste developer token (JWT, starts with eyJ...): ")
			devInput, err := reader.ReadString('\n')
			if err != nil {
				return err
			}
			devInput = strings.TrimSpace(devInput)
			if devInput == "" {
				fmt.Fprintln(cmd.OutOrStdout(), "Authentication cancelled.")
				return nil
			}

			fmt.Fprint(cmd.OutOrStdout(), "Paste user token (starts with Av...): ")
			userInput, err := reader.ReadString('\n')
			if err != nil {
				return err
			}
			userInput = strings.TrimSpace(userInput)
			if userInput == "" {
				fmt.Fprintln(cmd.OutOrStdout(), "Authentication cancelled.")
				return nil
			}

			token := applemusic.Token{
				DeveloperToken: devInput,
				MusicUserToken: userInput,
				StorefrontID:   strings.TrimSpace(storefrontID),
				CreatedAt:      time.Now().UTC(),
			}

			if err := store.Save(token); err != nil {
				return err
			}

			if isJSON(flags) {
				return writeJSON(cmd, map[string]any{
					"status":       "authenticated",
					"storefrontId": token.StorefrontID,
					"expiresAt":    token.ExpiresAt,
					"tokenPath":    store.Path(),
				})
			}

			fmt.Fprintln(cmd.OutOrStdout(), "Apple Music tokens saved successfully!")
			fmt.Fprintf(cmd.OutOrStdout(), "Token expires around: %s\n", token.ExpiresAt.Format(time.RFC3339))
			return nil
		},
	}

	cmd.Flags().StringVar(&developerToken, "developer-token", "", "Developer token (JWT)")
	cmd.Flags().StringVar(&userToken, "user-token", "", "Music user token")
	cmd.Flags().StringVar(&storefrontID, "storefront", "us", "Apple Music storefront ID (e.g., us, gb, jp)")

	return cmd
}

func newAuthAppleMusicStatusCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Check Apple Music authentication status",
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := applemusic.NewDefaultTokenStore()
			if err != nil {
				return err
			}

			token, ok, err := store.Load()
			if err != nil {
				return err
			}

			if !ok {
				if isJSON(flags) {
					return writeJSON(cmd, map[string]any{
						"authenticated": false,
						"tokenPath":     store.Path(),
					})
				}
				fmt.Fprintln(cmd.OutOrStdout(), "Not authenticated with Apple Music.")
				fmt.Fprintf(cmd.OutOrStdout(), "Run 'sonos auth applemusic login' to sign in.\n")
				return nil
			}

			expired := token.IsExpired()
			valid := token.IsValid()

			if isJSON(flags) {
				return writeJSON(cmd, map[string]any{
					"authenticated": true,
					"valid":         valid,
					"expired":       expired,
					"storefrontId":  token.StorefrontID,
					"createdAt":     token.CreatedAt,
					"expiresAt":     token.ExpiresAt,
					"tokenPath":     store.Path(),
				})
			}

			if valid {
				fmt.Fprintln(cmd.OutOrStdout(), "Authenticated with Apple Music.")
				fmt.Fprintf(cmd.OutOrStdout(), "Storefront: %s\n", token.StorefrontID)
				fmt.Fprintf(cmd.OutOrStdout(), "Created: %s\n", token.CreatedAt.Format(time.RFC3339))
				fmt.Fprintf(cmd.OutOrStdout(), "Expires: %s\n", token.ExpiresAt.Format(time.RFC3339))
			} else if expired {
				fmt.Fprintln(cmd.OutOrStdout(), "Apple Music token has expired.")
				fmt.Fprintf(cmd.OutOrStdout(), "Run 'sonos auth applemusic login' to re-authenticate.\n")
			} else {
				fmt.Fprintln(cmd.OutOrStdout(), "Apple Music token is invalid.")
				fmt.Fprintf(cmd.OutOrStdout(), "Run 'sonos auth applemusic login' to re-authenticate.\n")
			}

			return nil
		},
	}
}

func newAuthAppleMusicLogoutCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "logout",
		Short: "Remove Apple Music authentication",
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := applemusic.NewDefaultTokenStore()
			if err != nil {
				return err
			}

			if err := store.Delete(); err != nil {
				return err
			}

			if isJSON(flags) {
				return writeJSON(cmd, map[string]any{
					"status": "logged_out",
				})
			}

			fmt.Fprintln(cmd.OutOrStdout(), "Apple Music token removed.")
			return nil
		},
	}
}
