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
	Short: "Ingest a CSV batch of transactions via a JSON-RPC batch request",
	Long: "Reads a CSV file (header: " + ingest.ExpectedHeader + "), builds a JSON-RPC 2.0\n" +
		"batch — one element per row — and sends it to the server. Each row is applied\n" +
		"through the same transaction method the API uses, so idempotency, the overdraft\n" +
		"floor, and the audit trail are inherited.\n\n" +
		"With --dry-run the batch is built and printed but never sent.",
	RunE: runIngest,
}

func init() {
	ingestCmd.Flags().StringVar(&filePath, "file", "", "path to the CSV batch file (required)")
	ingestCmd.Flags().BoolVar(&dryRun, "dry-run", false, "build and print the batch without sending it")
	ingestCmd.Flags().StringVar(&method, "method", "Wallet.ProcessTransaction", "JSON-RPC method to call per row")
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

	batch := ingest.BuildBatch(rows, method)
	name := filepath.Base(filePath)
	out := cmd.OutOrStdout()

	if dryRun {
		encoded, err := json.MarshalIndent(batch, "", "  ")
		if err != nil {
			return fmt.Errorf("could not encode batch: %w", err)
		}
		fmt.Fprintln(out, string(encoded))
		fmt.Fprint(out, ingest.Summarize(name, rows, rowErrors, nil).Format(true))
		return nil
	}

	responses, err := ingest.Send(cmd.Context(), http.DefaultClient, serverURL, token, batch)
	if err != nil {
		return err
	}

	summary := ingest.Summarize(name, rows, rowErrors, responses)
	fmt.Fprint(out, summary.Format(false))
	return nil
}
