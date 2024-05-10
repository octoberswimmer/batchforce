package batch

import (
	"context"
	"fmt"
	"io"
	"time"

	force "github.com/ForceCLI/force/lib"
	csvmap "github.com/recursionpharma/go-csv-map"
	log "github.com/sirupsen/logrus"
)

func RecordsFromCsv(ctx context.Context, input io.Reader, processor chan<- force.ForceRecord) error {
	defer close(processor)

	var err error
	r := csvmap.NewReader(input)
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
	RECORD:
		for {
			select {
			case <-ctx.Done():
				return fmt.Errorf("Canceled: %w", ctx.Err())
			case processor <- r:
				// For WASM, allow event loop to continue
				// See https://github.com/golang/go/issues/39620#issuecomment-645513088
				time.Sleep(1 * time.Nanosecond)
				break RECORD
			case <-time.After(1 * time.Second):
				log.Info("Waiting to send record to converter")
			}
		}
	}

	return nil
}
