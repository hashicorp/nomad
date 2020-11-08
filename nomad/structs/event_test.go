package structs

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEventSink_Valid(t *testing.T) {
	cases := []struct {
		desc string
		e    *EventSink
		err  error
	}{
		{
			desc: "valid",
			e: &EventSink{

				ID:      "sink",
				Type:    SinkWebhook,
				Address: "http://127.0.0.1/",
				Topics: map[Topic][]string{
					TopicAll: {"*"},
				},
			},
		},
		{
			desc: "bad type",
			e: &EventSink{
				ID:      "sink",
				Type:    "custom",
				Address: "http://127.0.0.1/",
				Topics: map[Topic][]string{
					TopicAll: {"*"},
				},
			},
			err: fmt.Errorf("Sink type invalid"),
		},
		{
			desc: "bad ID",
			e: &EventSink{
				ID:      "sink id",
				Type:    SinkWebhook,
				Address: "http://127.0.0.1/",
				Topics: map[Topic][]string{
					TopicAll: {"*"},
				},
			},
			err: fmt.Errorf("Sink ID contains a space"),
		},
	}

	for _, tc := range cases {
		t.Run(tc.desc, func(t *testing.T) {
			err := tc.e.Validate()
			if tc.err != nil {
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.err.Error())
				return
			}
			require.NoError(t, err)
		})
	}

}

func TestEventSink_Changed(t *testing.T) {
	a := &EventSink{
		ID:      "sink",
		Type:    SinkWebhook,
		Address: "http://127.0.0.1/",
		Topics: map[Topic][]string{
			TopicAll: {"*"},
		},
	}
	b := new(EventSink)
	*b = *a
	require.True(t, b.EqualSubscriptionValues(a))

	b.Address = "http://127.0.0.1:8080/sink"
	require.False(t, b.EqualSubscriptionValues(a))

	c := new(EventSink)
	*c = *a
	c.Topics = make(map[Topic][]string)
	c.Topics["Deployment"] = []string{"5bccc81a-2514-48d3-890b-03bea3c84856"}
	require.False(t, c.EqualSubscriptionValues(a))
}

func Test_AWSEventBridgeSinkAddressValidator(t *testing.T) {
	type args struct {
	}
	var tests = []struct {
		name       string
		address    string
		wantBusArn string
		wantSource string
		wantErr    bool
	}{
		{"default",
			"arn:aws:events:us-east-1:123456789012:event-bus/default//nomad-cluster-main",
			"arn:aws:events:us-east-1:123456789012:event-bus/default",
			"nomad-cluster-main",
			false},
		{"badArn",
			"badArn//nomad-cluster-main",
			"badArn",
			"",
			true,
		},
		{"noSource",
			"arn:aws:events:us-east-1:123456789012:event-bus/default",
			"",
			"",
			true,
		},
		{"badSource",
			"arn:aws:events:us-east-1:123456789012:event-bus/default//",
			"arn:aws:events:us-east-1:123456789012:event-bus/default",
			"",
			true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			busArn, source, err := SinkAWSEventBridgeSinkAddressValidator(tt.address)
			if (err != nil) != tt.wantErr {
				t.Errorf("AWSEventBridgeSinkAddressValidator() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if busArn != tt.wantBusArn {
				t.Errorf("AWSEventBridgeSinkAddressValidator() busArn = %v, want %v", busArn, tt.wantBusArn)
			}
			if source != tt.wantSource {
				t.Errorf("AWSEventBridgeSinkAddressValidator() source = %v, want %v", source, tt.wantSource)
			}
		})
	}
}
