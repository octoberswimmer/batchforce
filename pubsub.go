//go:build !wasm

package batch

import (
	"context"

	force "github.com/ForceCLI/force/lib"
	"github.com/ForceCLI/force/lib/pubsub"
	log "github.com/sirupsen/logrus"
)

type uncheckableResult struct {
}

func (u uncheckableResult) NumberBatchesFailed() int {
	return 0
}

func (u uncheckableResult) NumberRecordsFailed() int {
	return 0
}

func PublishTo(session *force.Force, eventChannel string) RecordWriter {
	return func(ctx context.Context, records <-chan force.ForceRecord) (Result, error) {
		force.Log = log.StandardLogger()
		res := uncheckableResult{}
		err := pubsub.PublishMessagesWithContext(ctx, session, eventChannel, records)
		return res, err
	}
}
