package api

import (
	"time"
)

// Evaluation is used to serialize an evaluation.
type Evaluation struct {
	ID                string
	Priority          int
	Type              string
	TriggeredBy       string
	JobID             string
	JobModifyIndex    uint64
	NodeID            string
	NodeModifyIndex   uint64
	Status            string
	StatusDescription string
	Wait              time.Duration
	NextEval          string
	PreviousEval      string
}
