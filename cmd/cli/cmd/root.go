// Package cmd holds the loyalty-cli cobra command tree.
package cmd

import "github.com/spf13/cobra"

// Persistent flags shared by every subcommand that talks to the server.
var (
	serverURL string
	token     string
)

var rootCmd = &cobra.Command{
	Use:   "loyalty-cli",
	Short: "Command-line client for the loyalty-points JSON-RPC API",
	Long: "loyalty-cli is a thin client for the loyalty-points server. It talks to the\n" +
		"same JSON-RPC endpoint the API exposes; all business rules live on the server.",
	// RunE on subcommands handles its own errors; don't print usage on failure.
	SilenceUsage: true,
}

func init() {
	rootCmd.PersistentFlags().StringVar(&serverURL, "url", "http://localhost:8080/api", "JSON-RPC endpoint URL")
	rootCmd.PersistentFlags().StringVar(&token, "token", "", "Bearer access token for authenticated methods")

	rootCmd.AddCommand(ingestCmd)
}

// Execute runs the root command. main() turns a non-nil return into a non-zero
// exit code.
func Execute() error {
	return rootCmd.Execute()
}
