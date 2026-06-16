package cmd

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/Thatooine/loyalty-points-app/cmd/cli/pkg/ingest"
)

var (
	filePath string
	dryRun   bool
	method   string
)

var ingestCmd = &cobra.Command{
	Use:   "ingest",
	Short: "Ingest a CSV batch of transactions via a single JSON-RPC batch request",
	Long: "Reads a CSV file (header: " + ingest.ExpectedHeader + "), sorts the rows by\n" +
		"occurred_at (then line), and sends them as one ordered JSON-RPC request to the\n" +
		"wallet's admin-only batch method. The server applies them sequentially in that\n" +
		"order, so the overdraft floor sees transactions in their true chronology;\n" +
		"idempotency, the floor, and the audit trail are inherited from the same core\n" +
		"the single-transaction API uses.\n\n" +
		"With --dry-run the request is built and printed but never sent.",
	RunE: runIngest,
}

func init() {
	ingestCmd.Flags().StringVar(&filePath, "file", "", "path to the CSV batch file (required)")
	ingestCmd.Flags().BoolVar(&dryRun, "dry-run", false, "build and print the request without sending it")
	ingestCmd.Flags().StringVar(&method, "method", "Wallet.ProcessTransactionBatch", "JSON-RPC batch method to call")
	_ = ingestCmd.MarkFlagRequired("file")
}

func runIngest(cmd *cobra.Command, _ []string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("could not open %s: %w", filePath, err)
	}
	defer file.Close()

	rows, rowErrors, err := ingest.ParseCSV(file)
	if err != nil {
		return err
	}

	// Sort into the chronology the server applies in. The server is a faithful
	// sequential executor of this order; ordering policy lives here in the CLI.
	ingest.SortRows(rows)

	request := ingest.BuildRequest(rows, method)
	name := filepath.Base(filePath)
	out := cmd.OutOrStdout()

	if dryRun {
		encoded, err := json.MarshalIndent(request, "", "  ")
		if err != nil {
			return fmt.Errorf("could not encode request: %w", err)
		}
		fmt.Fprintln(out, string(encoded))
		fmt.Fprint(out, ingest.Summarize(name, rows, rowErrors, nil).Format(true))
		fmt.Fprint(out, ingest.Preview(rows))
		return nil
	}

	response, err := ingest.Send(cmd.Context(), http.DefaultClient, serverURL, token, request)
	if err != nil {
		return err
	}

	summary := ingest.Summarize(name, rows, rowErrors, &response)
	fmt.Fprint(out, summary.Format(false))
	return nil
}
