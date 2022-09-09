// Command ec2info provides a tool for generating a CPU performance lookup
// table indexed by EC2 instance types.
//
// By default the generated file will overwrite `env_aws_cpu.go` in Nomad's
// client/fingerprint package, when run from this directory.
//
// Requires AWS_ACCESS_KEY_ID, AWS_SECRET_ACCESS_KEY, AWS_SESSION_TOKEN.
//
// Usage (invoke from Nomad's makefile)
//
//	make ec2info
package main

import (
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"sort"
	"text/template"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
)

func check(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

func main() {
	pkg, region, output := "fingerprint", "us-west-1", "client/fingerprint/env_aws_cpu.go"

	client, err := clientForRegion(region)
	check(err)

	regions, err := getRegions(client)
	check(err)

	data, err := getData(regions)
	check(err)

	flat := flatten(data)

	f, err := open(output)
	check(err)
	defer func() {
		check(f.Close())
	}()

	check(write(f, flat, pkg))
	check(format(output))
}

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
		log.Println("failed to create AWS session; make sure environment is setup")
		log.Println("must have environment variables AWS_ACCESS_KEY_ID, AWS_SECRET_ACCESS_KEY, AWS_SESSION_TOKEN")
		log.Println("or ~/.aws/credentials configured properly")
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

func getData(regions []*ec2.Region) (map[string]map[string]specs, error) {
	data := make(map[string]map[string]specs)

	for _, region := range regions {
		rData, rProblems, err := getDataForRegion(*region.RegionName)
		if err != nil {
			return nil, err
		}
		data[*region.RegionName] = rData

		log.Println("region", *region.RegionName, "got data for", len(rData), "instance types", len(rProblems), "incomplete")
		instanceProblems(rProblems)
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

// open the output file for writing.
func open(output string) (io.ReadWriteCloser, error) {
	return os.Create(output)
}

// flatten region data, assuming instance type is the same across regions.
func flatten(data map[string]map[string]specs) map[string]specs {
	result := make(map[string]specs)
	for _, m := range data {
		for iType, specifications := range m {
			result[iType] = specifications
		}
	}
	return result
}

type Template struct {
	Package string
	Data    map[string]specs
}

// write the data using the cpu_table.go.template to w.
func write(w io.Writer, data map[string]specs, pkg string) error {
	tmpl, err := template.ParseFiles("tools/ec2info/cpu_table.go.template")
	if err != nil {
		return err
	}
	return tmpl.Execute(w, Template{
		Package: pkg,
		Data:    data,
	})
}

// format the file using gofmt.
func format(file string) error {
	cmd := exec.Command("gofmt", "-w", file)
	_, err := cmd.CombinedOutput()
	return err
}
