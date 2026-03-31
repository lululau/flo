package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
)

// Output writes data in the configured format (table or JSON).
func Output(data interface{}, headers []string, rows [][]string) error {
	if outputFormat == "json" {
		return outputJSON(data)
	}
	return outputTable(headers, rows)
}

func outputJSON(data interface{}) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(data)
}

func outputTable(headers []string, rows [][]string) error {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, strings.Join(headers, "\t"))
	for _, row := range rows {
		fmt.Fprintln(w, strings.Join(row, "\t"))
	}
	return w.Flush()
}
