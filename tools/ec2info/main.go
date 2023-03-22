// Command ec2info provides a tool for generating a CPU performance lookup
// table indexed by EC2 instance types.
//
// By default the generated file will overwrite `env_aws_cpu.go` in Nomad's
// client/fingerprint package, when run from this directory.
//
// Requires AWS_ACCESS_KEY_ID, AWS_SECRET_ACCESS_KEY, AWS_SESSION_TOKEN.
//
// Options
// --package : configure package name of generated output file
// --region : configure initial region from which to lookup all regions
// --outfile : configure filepath of generated output file
// --verbose : print log messages while running
//
// Usage
//   $ go run .
package main

import (
	"flag"
	"log"
)

func args() (string, string, string, bool) {
	pkg := flag.String("package", "fingerprint", "generate package name")
	region := flag.String("region", "us-west-1", "initial region for listing regions")
	outfile := flag.String("output", "../../client/fingerprint/env_aws_cpu.go", "output filepath")
	verbose := flag.Bool("verbose", true, "print extra information while running")
	flag.Parse()
	return *pkg, *region, *outfile, *verbose
}

func check(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

func main() {
	pkg, region, output, verbose := args()

	client, err := clientForRegion(region)
	check(err)

	regions, err := getRegions(client)
	check(err)

	data, err := getData(regions, verbose)
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
