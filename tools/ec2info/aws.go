package main

import (
	"fmt"
	"log"
	"sort"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
)

func clientForRegion(region string) (*ec2.EC2, error) {
	sess, err := session.NewSession(&aws.Config{
		Region: &region,
	})
	if err != nil {
		return nil, err
	}
	return ec2.New(sess), nil
}

func getRegions(client *ec2.EC2) ([]*ec2.Region, error) {
	all := false // beyond account access
	regions, err := client.DescribeRegions(&ec2.DescribeRegionsInput{
		AllRegions: &all,
	})
	if err != nil {
		return nil, err
	}
	return regions.Regions, nil
}

type specs struct {
	Cores int
	Speed float64
}

func (s specs) String() string {
	return fmt.Sprintf("(%d %.2f)", s.Cores, s.Speed)
}

func getData(regions []*ec2.Region, verbose bool) (map[string]map[string]specs, error) {
	data := make(map[string]map[string]specs)

	for _, region := range regions {
		rData, rProblems, err := getDataForRegion(*region.RegionName)
		if err != nil {
			return nil, err
		}
		data[*region.RegionName] = rData

		if verbose {
			log.Println("region", *region.RegionName, "got data for", len(rData), "instance types", len(rProblems), "incomplete")
			instanceProblems(rProblems)
		}
	}

	return data, nil
}

func instanceProblems(problems map[string]string) {
	types := make([]string, 0, len(problems))
	for k := range problems {
		types = append(types, k)
	}
	sort.Strings(types)
	for _, iType := range types {
		log.Println(" ->", iType, problems[iType])
	}
}

func getDataForRegion(region string) (map[string]specs, map[string]string, error) {
	client, err := clientForRegion(region)
	if err != nil {
		return nil, nil, err
	}

	data := make(map[string]specs)
	problems := make(map[string]string)
	regionInfoPage(client, true, region, nil, data, problems)
	return data, problems, nil
}

func regionInfoPage(client *ec2.EC2, first bool, region string, token *string, data map[string]specs, problems map[string]string) {
	if first || token != nil {
		output, err := client.DescribeInstanceTypes(&ec2.DescribeInstanceTypesInput{
			NextToken: token,
		})
		if err != nil {
			log.Fatal(err)
		}

		// recursively accumulate each page of data
		regionInfoAccumulate(output, data, problems)
		regionInfoPage(client, false, region, output.NextToken, data, problems)
	}
}

func regionInfoAccumulate(output *ec2.DescribeInstanceTypesOutput, data map[string]specs, problems map[string]string) {
	for _, iType := range output.InstanceTypes {
		switch {

		case iType.ProcessorInfo == nil:
			fallthrough
		case iType.ProcessorInfo.SustainedClockSpeedInGhz == nil:
			problems[*iType.InstanceType] = "missing clock Speed"
			continue

		case iType.VCpuInfo == nil:
			fallthrough
		case iType.VCpuInfo.DefaultVCpus == nil:
			problems[*iType.InstanceType] = "missing virtual cpu Cores"
			continue

		default:
			data[*iType.InstanceType] = specs{
				Speed: *iType.ProcessorInfo.SustainedClockSpeedInGhz,
				Cores: int(*iType.VCpuInfo.DefaultVCpus),
			}
			continue
		}
	}
}
