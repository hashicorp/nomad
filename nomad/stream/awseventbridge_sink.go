package stream

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/eventbridge"
	"github.com/hashicorp/go-multierror"
	"github.com/hashicorp/nomad/nomad/structs"
)

type AWSEventBridgeSink struct {
	client  *eventbridge.EventBridge
	busArn  string
	source  string
	Address string
}

func defaultEventBridgeClient() *eventbridge.EventBridge {
	newSession := session.Must(session.NewSession())
	svc := eventbridge.New(newSession)
	return svc
}

func NewAWSEventBridgeSink(eventSink *structs.EventSink) (SinkWriter, error) {
	busArn, source, err := structs.SinkAWSEventBridgeSinkAddressValidator(eventSink.Address)
	if err != nil {
		return nil, err
	}
	client := defaultEventBridgeClient()

	return &AWSEventBridgeSink{
		Address: eventSink.Address,
		busArn:  busArn,
		source:  source,
		client:  client,
	}, nil
}

func (ebs *AWSEventBridgeSink) Send(ctx context.Context, e *structs.Events) error {
	req, err := ebs.toRequest(e)
	if err != nil {
		return fmt.Errorf("converting event to request: %w", err)
	}

	err = ebs.doRequest(ctx, req)
	if err != nil {
		return fmt.Errorf("sending request to aws event bridge %w", err)
	}

	return nil
}

func (ebs *AWSEventBridgeSink) toRequest(events *structs.Events) (*eventbridge.PutEventsInput, error) {
	req := &eventbridge.PutEventsInput{}
	for _, e := range events.Events {
		buf := bytes.NewBuffer(nil)
		enc := json.NewEncoder(buf)
		if err := enc.Encode(e); err != nil {
			return nil, fmt.Errorf("error encoding event '%v' : %w", e, err)
		}
		req.Entries = append(req.Entries, &eventbridge.PutEventsRequestEntry{
			Detail: aws.String(buf.String()),
			// TODO: reflect on structs.Event fields
			DetailType:   aws.String("Topic Type Key Namespace FilterKeys Index Payload"),
			EventBusName: aws.String(ebs.busArn),
			Source:       aws.String(ebs.source),
		})
	}
	return req, nil
}

func (ebs *AWSEventBridgeSink) doRequest(ctx context.Context, req *eventbridge.PutEventsInput) error {
	resp, err := ebs.client.PutEventsWithContext(ctx, req)
	if err != nil {
		return err
	}
	return ebs.respErrorCheck(resp)
}

func (ebs *AWSEventBridgeSink) respErrorCheck(resp *eventbridge.PutEventsOutput) error {
	var entryErrors error
	if *resp.FailedEntryCount > 0 {
		entryErrors = multierror.Append(entryErrors, fmt.Errorf("found %d entry errors", *resp.FailedEntryCount))
		for _, e := range resp.Entries {
			if e.ErrorCode != nil {
				entryErrors = multierror.Append(entryErrors,
					fmt.Errorf(
						"entry '%s' failed with code '%s' and msg '%s'",
						*e.EventId,
						*e.ErrorCode,
						*e.ErrorMessage))
			}
		}
	}
	return entryErrors
}
