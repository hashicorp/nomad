package stream

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/eventbridge"
	"testing"
)

func TestAWSEventBridgeSink_respErrorCheck(t *testing.T) {
	tests := []struct {
		name    string
		resp    *eventbridge.PutEventsOutput
		wantErr bool
	}{
		{"no error",
			&eventbridge.PutEventsOutput{
				FailedEntryCount: aws.Int64(0),
			},
			false,
		},
		{"should error",
			&eventbridge.PutEventsOutput{
				FailedEntryCount: aws.Int64(1),
				Entries: []*eventbridge.PutEventsResultEntry{
					{
						ErrorCode:    nil,
						ErrorMessage: nil,
						EventId:      nil,
					},
					{
						ErrorCode:    aws.String("Validate"),
						ErrorMessage: aws.String("Cannot do cross-region event puts"),
						EventId:      aws.String("event-id"),
					},
				},
			},
			true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ebs := &AWSEventBridgeSink{
				client:  nil,
				busArn:  "notUnderTest",
				source:  "notUnderTest",
				Address: "notUnderTest",
			}
			if err := ebs.respErrorCheck(tt.resp); (err != nil) != tt.wantErr {
				t.Errorf("respErrorCheck() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
