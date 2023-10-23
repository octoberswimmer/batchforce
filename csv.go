package batch

import (
	"context"
	"fmt"
	"io"
	"os"

	force "github.com/ForceCLI/force/lib"
	csvmap "github.com/recursionpharma/go-csv-map"
)

func recordsFromCsv(ctx context.Context, fileName string, processor chan<- force.ForceRecord) error {
	defer close(processor)

	f, err := os.Open(fileName)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	r := csvmap.NewReader(f)
	r.Columns, err = r.ReadHeader()
	if err != nil {
		return fmt.Errorf("failed to read csv header: %w", err)
	}

	for {
		row, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("failed to read csv row: %w", err)
		}
		r := force.ForceRecord{}
		for k, v := range row {
			r[k] = any(v)
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("Cancelled: %w", ctx.Err())
		default:
			processor <- r
		}
	}

	return nil
}
