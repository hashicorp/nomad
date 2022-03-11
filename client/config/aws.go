package config

import (
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/cloudwatch"
	"github.com/hashicorp/nomad/helper"
	"time"
)

func GetFoo() error {
	start := "2022-03-09T00:00:00Z"
	end := "2022-03-10T00:00:00Z"

	startTime, err := time.Parse(time.RFC3339, start)
	if err != nil {
		return err
	}
	endTime, err := time.Parse(time.RFC3339, end)
	if err != nil {
		return err
	}

	// You must have a ~/.aws/credentials file in this format with values from doormat
	// [default]
	// aws_access_key_id = <YOUR ACCESS KEY ID>
	// aws_secret_access_key = <YOUR SECRET ACCESS KEY>
	// aws_session_token =  <YOUR SESSION TOKEN>
	sess, err := session.NewSessionWithOptions(session.Options{
		SharedConfigState: session.SharedConfigEnable,
		Config:            aws.Config{Region: aws.String("us-east-1")},
	})
	if err != nil {
		fmt.Println(err)
		return err
	}

	svc := cloudwatch.New(sess)

	metricResult, err := svc.GetMetricData(&cloudwatch.GetMetricDataInput{
		StartTime: &startTime,
		EndTime:   &endTime,
		ScanBy:    helper.StringToPtr("TimestampAscending"),
		MetricDataQueries: []*cloudwatch.MetricDataQuery{
			{
				Id:         helper.StringToPtr("cpuUtilizationWithEmptyValues"),
				Expression: helper.StringToPtr("SEARCH('{AWS/EC2,InstanceId} MetricName=\"CPUUtilization\"', 'Average', 3600)"),
				ReturnData: helper.BoolToPtr(false),
			},
			{
				Id:         helper.StringToPtr("cpuUtilization"),
				Expression: helper.StringToPtr("REMOVE_EMPTY(cpuUtilizationWithEmptyValues)"),
			},
			{
				Id:         helper.StringToPtr("vCPUs"),
				Expression: helper.StringToPtr("SEARCH('{AWS/Usage,Resource,Type,Service,Class } Resource=\"vCPU\" MetricName=\"ResourceCount\"', 'Average', 3600)"),
			},
		},
	},
	)

	if err != nil {
		return err
	}

	fmt.Printf("%#v", metricResult)

	return nil
}
