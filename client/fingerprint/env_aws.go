package fingerprint

import (
	"fmt"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/ec2metadata"
	"github.com/aws/aws-sdk-go/aws/session"
	log "github.com/hashicorp/go-hclog"

	cleanhttp "github.com/hashicorp/go-cleanhttp"
	"github.com/hashicorp/nomad/nomad/structs"
)

const (
	// AwsMetadataTimeout is the timeout used when contacting the AWS metadata
	// services.
	AwsMetadataTimeout = 2 * time.Second
)

// map of instance type to approximate speed, in Mbits/s
// Estimates from http://stackoverflow.com/a/35806587
// This data is meant for a loose approximation
var ec2NetSpeedTable = map[*regexp.Regexp]int{
	regexp.MustCompile("t2.nano"):      30,
	regexp.MustCompile("t2.micro"):     70,
	regexp.MustCompile("t2.small"):     125,
	regexp.MustCompile("t2.medium"):    300,
	regexp.MustCompile("m3.medium"):    400,
	regexp.MustCompile("c4.8xlarge"):   4000,
	regexp.MustCompile("x1.16xlarge"):  5000,
	regexp.MustCompile(`.*\.large`):    500,
	regexp.MustCompile(`.*\.xlarge`):   750,
	regexp.MustCompile(`.*\.2xlarge`):  1000,
	regexp.MustCompile(`.*\.4xlarge`):  2000,
	regexp.MustCompile(`.*\.8xlarge`):  10000,
	regexp.MustCompile(`.*\.10xlarge`): 10000,
	regexp.MustCompile(`.*\.16xlarge`): 10000,
	regexp.MustCompile(`.*\.32xlarge`): 10000,
}

type ec2Specs struct {
	mhz   float64
	cores int
	model string
}

func (e ec2Specs) ticks() int {
	return int(e.mhz) * e.cores
}

func specs(ghz float64, vCores int, model string) ec2Specs {
	return ec2Specs{
		mhz:   ghz * 1000,
		cores: vCores,
		model: model,
	}
}

// Map of instance type to documented CPU speed.
//
// Most values are taken from https://aws.amazon.com/ec2/instance-types/.
// Values for a1 & m6g (Graviton) are taken from https://en.wikichip.org/wiki/annapurna_labs/alpine/al73400
// Values for inf1 are taken from launching a inf1.xlarge and looking at /proc/cpuinfo
//
// In a few cases, AWS has upgraded the generation of CPU while keeping the same
// instance designation. Since it is possible to launch on the lower performance
// CPU, that one is used as the spec for the instance type.
//
// This table is provided as a best-effort to determine the number of CPU ticks
// available for use by Nomad tasks. If an instance type is missing, the fallback
// behavior is to use values from go-psutil, which is only capable of reading
// "current" CPU MHz.
var ec2ProcSpeedTable = map[string]ec2Specs{
	// -- General Purpose --

	// a1
	"a1.medium":  specs(2.3, 1, "AWS Graviton"),
	"a1.large":   specs(2.3, 2, "AWS Graviton"),
	"a1.xlarge":  specs(2.3, 4, "AWS Graviton"),
	"a1.2xlarge": specs(2.3, 8, "AWS Graviton"),
	"a1.4xlarge": specs(2.3, 16, "AWS Graviton"),
	"a1.metal":   specs(2.3, 16, "AWS Graviton"),

	// t3
	"t3.nano":    specs(2.5, 2, "2.5 GHz Intel Scalable"),
	"t3.micro":   specs(2.5, 2, "2.5 GHz Intel Scalable"),
	"t3.small":   specs(2.5, 2, "2.5 GHz Intel Scalable"),
	"t3.medium":  specs(2.5, 2, "2.5 GHz Intel Scalable"),
	"t3.large":   specs(2.5, 2, "2.5 GHz Intel Scalable"),
	"t3.xlarge":  specs(2.5, 4, "2.5 GHz Intel Scalable"),
	"t3.2xlarge": specs(2.5, 8, "2.5 GHz Intel Scalable"),

	// t3a
	"t3a.nano":    specs(2.5, 2, "2.5 GHz AMD EPYC 7000 series"),
	"t3a.micro":   specs(2.5, 2, "2.5 GHz AMD EPYC 7000 series"),
	"t3a.small":   specs(2.5, 2, "2.5 GHz AMD EPYC 7000 series"),
	"t3a.medium":  specs(2.5, 2, "2.5 GHz AMD EPYC 7000 series"),
	"t3a.large":   specs(2.5, 2, "2.5 GHz AMD EPYC 7000 series"),
	"t3a.xlarge":  specs(2.5, 4, "2.5 GHz AMD EPYC 7000 series"),
	"t3a.2xlarge": specs(2.5, 8, "2.5 GHz AMD EPYC 7000 series"),

	// t2
	"t2.nano":    specs(3.3, 1, "3.3 GHz Intel Scalable"),
	"t2.micro":   specs(3.3, 1, "3.3 GHz Intel Scalable"),
	"t2.small":   specs(3.3, 1, "3.3 GHz Intel Scalable"),
	"t2.medium":  specs(3.3, 2, "3.3 GHz Intel Scalable"),
	"t2.large":   specs(3.0, 2, "3.0 GHz Intel Scalable"),
	"t2.xlarge":  specs(3.0, 4, "3.0 GHz Intel Scalable"),
	"t2.2xlarge": specs(3.0, 8, "3.0 GHz Intel Scalable"),

	// m6g
	"m6g.medium":   specs(2.3, 1, "AWS Graviton2 Neoverse"),
	"m6g.large":    specs(2.3, 2, "AWS Graviton2 Neoverse"),
	"m6g.xlarge":   specs(2.3, 4, "AWS Graviton2 Neoverse"),
	"m6g.2xlarge":  specs(2.3, 8, "AWS Graviton2 Neoverse"),
	"m6g.4xlarge":  specs(2.3, 16, "AWS Graviton2 Neoverse"),
	"m6g.8xlarge":  specs(2.3, 32, "AWS Graviton2 Neoverse"),
	"m6g.12xlarge": specs(2.3, 48, "AWS Graviton2 Neoverse"),
	"m6g.16xlarge": specs(2.3, 64, "AWS Graviton2 Neoverse"),

	// m5, m5d
	"m5.large":     specs(3.1, 2, "3.1 GHz Intel Xeon Platinum"),
	"m5.xlarge":    specs(3.1, 4, "3.1 GHz Intel Xeon Platinum"),
	"m5.2xlarge":   specs(3.1, 8, "3.1 GHz Intel Xeon Platinum"),
	"m5.4xlarge":   specs(3.1, 16, "3.1 GHz Intel Xeon Platinum"),
	"m5.8xlarge":   specs(3.1, 32, "3.1 GHz Intel Xeon Platinum"),
	"m5.12xlarge":  specs(3.1, 48, "3.1 GHz Intel Xeon Platinum"),
	"m5.16xlarge":  specs(3.1, 64, "3.1 GHz Intel Xeon Platinum"),
	"m5.24xlarge":  specs(3.1, 96, "3.1 GHz Intel Xeon Platinum"),
	"m5.metal":     specs(3.1, 96, "3.1 GHz Intel Xeon Platinum"),
	"m5d.large":    specs(3.1, 2, "3.1 GHz Intel Xeon Platinum"),
	"m5d.xlarge":   specs(3.1, 4, "3.1 GHz Intel Xeon Platinum"),
	"m5d.2xlarge":  specs(3.1, 8, "3.1 GHz Intel Xeon Platinum"),
	"m5d.4xlarge":  specs(3.1, 16, "3.1 GHz Intel Xeon Platinum"),
	"m5d.8xlarge":  specs(3.1, 32, "3.1 GHz Intel Xeon Platinum"),
	"m5d.12xlarge": specs(3.1, 48, "3.1 GHz Intel Xeon Platinum"),
	"m5d.16xlarge": specs(3.1, 64, "3.1 GHz Intel Xeon Platinum"),
	"m5d.24xlarge": specs(3.1, 96, "3.1 GHz Intel Xeon Platinum"),
	"m5d.metal":    specs(3.1, 96, "3.1 GHz Intel Xeon Platinum"),

	// m5a, m5ad
	"m5a.large":     specs(2.5, 2, "2.5 GHz AMD EPYC 7000 series"),
	"m5a.xlarge":    specs(2.5, 4, "2.5 GHz AMD EPYC 7000 series"),
	"m5a.2xlarge":   specs(2.5, 8, "2.5 GHz AMD EPYC 7000 series"),
	"m5a.4xlarge":   specs(2.5, 16, "2.5 GHz AMD EPYC 7000 series"),
	"m5a.8xlarge":   specs(2.5, 32, "2.5 GHz AMD EPYC 7000 series"),
	"m5a.12xlarge":  specs(2.5, 48, "2.5 GHz AMD EPYC 7000 series"),
	"m5a.16xlarge":  specs(2.5, 64, "2.5 GHz AMD EPYC 7000 series"),
	"m5a.24xlarge":  specs(2.5, 96, "2.5 GHz AMD EPYC 7000 series"),
	"m5ad.large":    specs(2.5, 2, "2.5 GHz AMD EPYC 7000 series"),
	"m5ad.xlarge":   specs(2.5, 4, "2.5 GHz AMD EPYC 7000 series"),
	"m5ad.2xlarge":  specs(2.5, 8, "2.5 GHz AMD EPYC 7000 series"),
	"m5ad.4xlarge":  specs(2.5, 16, "2.5 GHz AMD EPYC 7000 series"),
	"m5ad.12xlarge": specs(2.5, 48, "2.5 GHz AMD EPYC 7000 series"),
	"m5ad.24xlarge": specs(2.5, 96, "2.5 GHz AMD EPYC 7000 series"),

	// m5n, m5dn
	"m5n.large":     specs(3.1, 2, "3.1 GHz Intel Xeon Scalable"),
	"m5n.xlarge":    specs(3.1, 4, "3.1 GHz Intel Xeon Scalable"),
	"m5n.2xlarge":   specs(3.1, 8, "3.1 GHz Intel Xeon Scalable"),
	"m5n.4xlarge":   specs(3.1, 16, "3.1 GHz Intel Xeon Scalable"),
	"m5n.8xlarge":   specs(3.1, 32, "3.1 GHz Intel Xeon Scalable"),
	"m5n.12xlarge":  specs(3.1, 48, "3.1 GHz Intel Xeon Scalable"),
	"m5n.16xlarge":  specs(3.1, 64, "3.1 GHz Intel Xeon Scalable"),
	"m5n.24xlarge":  specs(3.1, 96, "3.1 GHz Intel Xeon Scalable"),
	"m5dn.large":    specs(3.1, 2, "3.1 GHz Intel Xeon Scalable"),
	"m5dn.xlarge":   specs(3.1, 4, "3.1 GHz Intel Xeon Scalable"),
	"m5dn.2xlarge":  specs(3.1, 8, "3.1 GHz Intel Xeon Scalable"),
	"m5dn.4xlarge":  specs(3.1, 16, "3.1 GHz Intel Xeon Scalable"),
	"m5dn.8xlarge":  specs(3.1, 32, "3.1 GHz Intel Xeon Scalable"),
	"m5dn.12xlarge": specs(3.1, 48, "3.1 GHz Intel Xeon Scalable"),
	"m5dn.16xlarge": specs(3.1, 64, "3.1 GHz Intel Xeon Scalable"),
	"m5dn.24xlarge": specs(3.1, 96, "3.1 GHz Intel Xeon Scalable"),

	// m4
	"m4.large":    specs(2.3, 2, "2.3 GHz Intel Xeon® E5-2686 v4"),
	"m4.xlarge":   specs(2.3, 4, "2.3 GHz Intel Xeon® E5-2686 v4"),
	"m4.2xlarge":  specs(2.3, 8, "2.3 GHz Intel Xeon® E5-2686 v4"),
	"m4.4xlarge":  specs(2.3, 16, "2.3 GHz Intel Xeon® E5-2686 v4"),
	"m4.10xlarge": specs(2.3, 40, "2.3 GHz Intel Xeon® E5-2686 v4"),
	"m4.16xlarge": specs(2.3, 64, "2.3 GHz Intel Xeon® E5-2686 v4"),

	// -- Compute Optimized --

	// c5, c5d
	"c5.large":     specs(3.4, 2, "3.4 GHz Intel Xeon Platinum 8000"),
	"c5.xlarge":    specs(3.4, 4, "3.4 GHz Intel Xeon Platinum 8000"),
	"c5.2xlarge":   specs(3.4, 8, "3.4 GHz Intel Xeon Platinum 8000"),
	"c5.4xlarge":   specs(3.4, 16, "3.4 GHz Intel Xeon Platinum 8000"),
	"c5.9xlarge":   specs(3.4, 36, "3.4 GHz Intel Xeon Platinum 8000"),
	"c5.12xlarge":  specs(3.6, 48, "3.6 GHz Intel Xeon Scalable"),
	"c5.18xlarge":  specs(3.6, 72, "3.6 GHz Intel Xeon Scalable"),
	"c5.24xlarge":  specs(3.6, 96, "3.6 GHz Intel Xeon Scalable"),
	"c5.metal":     specs(3.6, 96, "3.6 GHz Intel Xeon Scalable"),
	"c5d.large":    specs(3.4, 2, "3.4 GHz Intel Xeon Platinum 8000"),
	"c5d.xlarge":   specs(3.4, 4, "3.4 GHz Intel Xeon Platinum 8000"),
	"c5d.2xlarge":  specs(3.4, 8, "3.4 GHz Intel Xeon Platinum 8000"),
	"c5d.4xlarge":  specs(3.4, 16, "3.4 GHz Intel Xeon Platinum 8000"),
	"c5d.9xlarge":  specs(3.4, 36, "3.4 GHz Intel Xeon Platinum 8000"),
	"c5d.12xlarge": specs(3.6, 48, "3.6 GHz Intel Xeon Scalable"),
	"c5d.18xlarge": specs(3.6, 72, "3.6 GHz Intel Xeon Scalable"),
	"c5d.24xlarge": specs(3.6, 96, "3.6 GHz Intel Xeon Scalable"),
	"c5d.metal":    specs(3.6, 96, "3.6 GHz Intel Xeon Scalable"),

	// c5n
	"c5n.large":    specs(3.0, 2, "3.0 GHz Intel Xeon Platinum"),
	"c5n.xlarge":   specs(3.0, 4, "3.0 GHz Intel Xeon Platinum"),
	"c5n.2xlarge":  specs(3.0, 8, "3.0 GHz Intel Xeon Platinum"),
	"c5n.4xlarge":  specs(3.0, 16, "3.0 GHz Intel Xeon Platinum"),
	"c5n.9xlarge":  specs(3.0, 36, "3.0 GHz Intel Xeon Platinum"),
	"c5n.18xlarge": specs(3.0, 72, "3.0 GHz Intel Xeon Platinum"),
	"c5n.metal":    specs(3.0, 72, "3.0 GHz Intel Xeon Platinum"),

	// c4
	"c4.large":   specs(2.9, 2, "2.9 GHz Intel Xeon E5-2666 v3"),
	"c4.xlarge":  specs(2.9, 4, "2.9 GHz Intel Xeon E5-2666 v3"),
	"c4.2xlarge": specs(2.9, 8, "2.9 GHz Intel Xeon E5-2666 v3"),
	"c4.4xlarge": specs(2.9, 16, "2.9 GHz Intel Xeon E5-2666 v3"),
	"c4.8xlarge": specs(2.9, 36, "2.9 GHz Intel Xeon E5-2666 v3"),

	// -- Memory Optimized --

	// r5, r5d
	"r5.large":     specs(3.1, 2, "3.1 GHz Intel Xeon Platinum 8175"),
	"r5.xlarge":    specs(3.1, 4, "3.1 GHz Intel Xeon Platinum 8175"),
	"r5.2xlarge":   specs(3.1, 8, "3.1 GHz Intel Xeon Platinum 8175"),
	"r5.4xlarge":   specs(3.1, 16, "3.1 GHz Intel Xeon Platinum 8175"),
	"r5.8xlarge":   specs(3.1, 32, "3.1 GHz Intel Xeon Platinum 8175"),
	"r5.12xlarge":  specs(3.1, 48, "3.1 GHz Intel Xeon Platinum 8175"),
	"r5.16xlarge":  specs(3.1, 64, "3.1 GHz Intel Xeon Platinum 8175"),
	"r5.24xlarge":  specs(3.1, 96, "3.1 GHz Intel Xeon Platinum 8175"),
	"r5.metal":     specs(3.1, 96, "3.1 GHz Intel Xeon Platinum 8175"),
	"r5d.large":    specs(3.1, 2, "3.1 GHz Intel Xeon Platinum 8175"),
	"r5d.xlarge":   specs(3.1, 4, "3.1 GHz Intel Xeon Platinum 8175"),
	"r5d.2xlarge":  specs(3.1, 8, "3.1 GHz Intel Xeon Platinum 8175"),
	"r5d.4xlarge":  specs(3.1, 16, "3.1 GHz Intel Xeon Platinum 8175"),
	"r5d.8xlarge":  specs(3.1, 32, "3.1 GHz Intel Xeon Platinum 8175"),
	"r5d.12xlarge": specs(3.1, 48, "3.1 GHz Intel Xeon Platinum 8175"),
	"r5d.16xlarge": specs(3.1, 64, "3.1 GHz Intel Xeon Platinum 8175"),
	"r5d.24xlarge": specs(3.1, 96, "3.1 GHz Intel Xeon Platinum 8175"),
	"r5d.metal":    specs(3.1, 96, "3.1 GHz Intel Xeon Platinum 8175"),

	// r5a, r5ad
	"r5a.large":     specs(2.5, 2, "2.5 GHz AMD EPYC 7000 series"),
	"r5a.xlarge":    specs(2.5, 4, "2.5 GHz AMD EPYC 7000 series"),
	"r5a.2xlarge":   specs(2.5, 8, "2.5 GHz AMD EPYC 7000 series"),
	"r5a.4xlarge":   specs(2.5, 16, "2.5 GHz AMD EPYC 7000 series"),
	"r5a.8xlarge":   specs(2.5, 32, "2.5 GHz AMD EPYC 7000 series"),
	"r5a.12xlarge":  specs(2.5, 48, "2.5 GHz AMD EPYC 7000 series"),
	"r5a.16xlarge":  specs(2.5, 64, "2.5 GHz AMD EPYC 7000 series"),
	"r5a.24xlarge":  specs(2.5, 96, "2.5 GHz AMD EPYC 7000 series"),
	"r5ad.large":    specs(2.5, 2, "2.5 GHz AMD EPYC 7000 series"),
	"r5ad.xlarge":   specs(2.5, 4, "2.5 GHz AMD EPYC 7000 series"),
	"r5ad.2xlarge":  specs(2.5, 8, "2.5 GHz AMD EPYC 7000 series"),
	"r5ad.4xlarge":  specs(2.5, 16, "2.5 GHz AMD EPYC 7000 series"),
	"r5ad.8xlarge":  specs(2.5, 32, "2.5 GHz AMD EPYC 7000 series"),
	"r5ad.12xlarge": specs(2.5, 48, "2.5 GHz AMD EPYC 7000 series"),
	"r5ad.16xlarge": specs(2.5, 64, "2.5 GHz AMD EPYC 7000 series"),
	"r5ad.24xlarge": specs(2.5, 96, "2.5 GHz AMD EPYC 7000 series"),

	// r5n
	"r5n.large":     specs(3.1, 2, "3.1 GHz Intel Xeon Scalable"),
	"r5n.xlarge":    specs(3.1, 4, "3.1 GHz Intel Xeon Scalable"),
	"r5n.2xlarge":   specs(3.1, 8, "3.1 GHz Intel Xeon Scalable"),
	"r5n.4xlarge":   specs(3.1, 16, "3.1 GHz Intel Xeon Scalable"),
	"r5n.8xlarge":   specs(3.1, 32, "3.1 GHz Intel Xeon Scalable"),
	"r5n.12xlarge":  specs(3.1, 48, "3.1 GHz Intel Xeon Scalable"),
	"r5n.16xlarge":  specs(3.1, 64, "3.1 GHz Intel Xeon Scalable"),
	"r5n.24xlarge":  specs(3.1, 96, "3.1 GHz Intel Xeon Scalable"),
	"r5dn.large":    specs(3.1, 2, "3.1 GHz Intel Xeon Scalable"),
	"r5dn.xlarge":   specs(3.1, 4, "3.1 GHz Intel Xeon Scalable"),
	"r5dn.2xlarge":  specs(3.1, 8, "3.1 GHz Intel Xeon Scalable"),
	"r5dn.4xlarge":  specs(3.1, 16, "3.1 GHz Intel Xeon Scalable"),
	"r5dn.8xlarge":  specs(3.1, 32, "3.1 GHz Intel Xeon Scalable"),
	"r5dn.12xlarge": specs(3.1, 48, "3.1 GHz Intel Xeon Scalable"),
	"r5dn.16xlarge": specs(3.1, 64, "3.1 GHz Intel Xeon Scalable"),
	"r5dn.24xlarge": specs(3.1, 96, "3.1 GHz Intel Xeon Scalable"),

	// r4
	"r4.large":    specs(2.3, 2, "2.3 GHz Intel Xeon E5-2686 v4"),
	"r4.xlarge":   specs(2.3, 4, "2.3 GHz Intel Xeon E5-2686 v4"),
	"r4.2xlarge":  specs(2.3, 8, "2.3 GHz Intel Xeon E5-2686 v4"),
	"r4.4xlarge":  specs(2.3, 16, "2.3 GHz Intel Xeon E5-2686 v4"),
	"r4.8xlarge":  specs(2.3, 32, "2.3 GHz Intel Xeon E5-2686 v4"),
	"r4.16xlarge": specs(2.3, 64, "2.3 GHz Intel Xeon E5-2686 v4"),

	// x1e
	"x1e.xlarge":   specs(2.3, 4, "2.3 GHz Intel Xeon E7-8880 v3"),
	"x1e.2xlarge":  specs(2.3, 8, "2.3 GHz Intel Xeon E7-8880 v3"),
	"x1e.4xlarge":  specs(2.3, 16, "2.3 GHz Intel Xeon E7-8880 v3"),
	"x1e.8xlarge":  specs(2.3, 32, "2.3 GHz Intel Xeon E7-8880 v3"),
	"x1e.16xlarge": specs(2.3, 64, "2.3 GHz Intel Xeon E7-8880 v3"),
	"x1e.32xlarge": specs(2.3, 128, "2.3 GHz Intel Xeon E7-8880 v3"),

	// x1
	"x1.16xlarge": specs(2.3, 64, "2.3 GHz Intel Xeon E7-8880 v3"),
	"x1.32xlarge": specs(2.3, 64, "2.3 GHz Intel Xeon E7-8880 v3"),

	// high-memory
	"u-6tb1.metal":  specs(2.1, 448, "2.1 GHz Intel Xeon Platinum 8176M"),
	"u-9tb1.metal":  specs(2.1, 448, "2.1 GHz Intel Xeon Platinum 8176M"),
	"u-12tb1.metal": specs(2.1, 448, "2.1 GHz Intel Xeon Platinum 8176M"),
	"u-18tb1.metal": specs(2.7, 448, "2.7 GHz Intel Xeon Scalable"),
	"u-24tb1.metal": specs(2.7, 448, "2.7 GHz Intel Xeon Scalable"),

	// z1d
	"z1d.large":    specs(4.0, 2, "4.0 GHz Intel Xeon Scalable"),
	"z1d.xlarge":   specs(4.0, 4, "4.0 GHz Intel Xeon Scalable"),
	"z1d.2xlarge":  specs(4.0, 8, "4.0 GHz Intel Xeon Scalable"),
	"z1d.3xlarge":  specs(4.0, 12, "4.0 GHz Intel Xeon Scalable"),
	"z1d.6xlarge":  specs(4.0, 24, "4.0 GHz Intel Xeon Scalable"),
	"z1d.12xlarge": specs(4.0, 48, "4.0 GHz Intel Xeon Scalable"),
	"z1d.metal":    specs(4.0, 48, "4.0 GHz Intel Xeon Scalable"),

	// -- Accelerated Computing --

	// p3, p3dn
	"p3.2xlarge":    specs(2.3, 8, "2.3 GHz Intel Xeon E5-2686 v4"),
	"p3.8xlarge":    specs(2.3, 32, "2.3 GHz Intel Xeon E5-2686 v4"),
	"p3.16xlarge":   specs(2.3, 64, "2.3 GHz Intel Xeon E5-2686 v4"),
	"p3dn.24xlarge": specs(2.5, 96, "2.5 GHz Intel Xeon P-8175M"),

	// p2
	"p2.xlarge":   specs(2.3, 4, "2.3 GHz Intel Xeon E5-2686 v4"),
	"p2.8xlarge":  specs(2.3, 32, "2.3 GHz Intel Xeon E5-2686 v4"),
	"p2.16xlarge": specs(2.3, 64, "2.3 GHz Intel Xeon E5-2686 v4"),

	// inf1
	"inf1.xlarge":   specs(3.0, 4, "3.0 GHz Intel Xeon Platinum 8275CL"),
	"inf1.2xlarge":  specs(3.0, 8, "3.0 GHz Intel Xeon Platinum 8275CL"),
	"inf1.6xlarge":  specs(3.0, 24, "3.0 GHz Intel Xeon Platinum 8275CL"),
	"inf1.24xlarge": specs(3.0, 96, "3.0 GHz Intel Xeon Platinum 8275CL"),

	// g4dn
	"g4dn.xlarge":   specs(2.5, 4, "2.5 GHz Cascade Lake 24C"),
	"g4dn.2xlarge":  specs(2.5, 8, "2.5 GHz Cascade Lake 24C"),
	"g4dn.4xlarge":  specs(2.5, 16, "2.5 GHz Cascade Lake 24C"),
	"g4dn.8xlarge":  specs(2.5, 32, "2.5 GHz Cascade Lake 24C"),
	"g4dn.16xlarge": specs(2.5, 64, "2.5 GHz Cascade Lake 24C"),
	"g4dn.12xlarge": specs(2.5, 48, "2.5 GHz Cascade Lake 24C"),
	"g4dn.metal":    specs(2.5, 96, "2.5 GHz Cascade Lake 24C"),

	// g3
	"g3s.xlarge":   specs(2.3, 4, "2.3 GHz Intel Xeon E5-2686 v4"),
	"g3s.4xlarge":  specs(2.3, 16, "2.3 GHz Intel Xeon E5-2686 v4"),
	"g3s.8xlarge":  specs(2.3, 32, "2.3 GHz Intel Xeon E5-2686 v4"),
	"g3s.16xlarge": specs(2.3, 64, "2.3 GHz Intel Xeon E5-2686 v4"),

	// f1
	"f1.2xlarge":  specs(2.3, 8, "Intel Xeon E5-2686 v4"),
	"f1.4xlarge":  specs(2.3, 16, "Intel Xeon E5-2686 v4"),
	"f1.16xlarge": specs(2.3, 64, "Intel Xeon E5-2686 v4"),

	// -- Storage Optimized --

	// i3
	"i3.large":    specs(2.3, 2, "2.3 GHz Intel Xeon E5 2686 v4"),
	"i3.xlarge":   specs(2.3, 4, "2.3 GHz Intel Xeon E5 2686 v4"),
	"i3.2xlarge":  specs(2.3, 8, "2.3 GHz Intel Xeon E5 2686 v4"),
	"i3.4xlarge":  specs(2.3, 16, "2.3 GHz Intel Xeon E5 2686 v4"),
	"i3.8xlarge":  specs(2.3, 32, "2.3 GHz Intel Xeon E5 2686 v4"),
	"i3.16xlarge": specs(2.3, 64, "2.3 GHz Intel Xeon E5 2686 v4"),
	"i3.metal":    specs(2.3, 72, "2.3 GHz Intel Xeon E5 2686 v4"),

	// i3en
	"i3en.large":    specs(3.1, 2, "3.1 GHz Intel Xeon Scalable"),
	"i3en.xlarge":   specs(3.1, 4, "3.1 GHz Intel Xeon Scalable"),
	"i3en.2xlarge":  specs(3.1, 8, "3.1 GHz Intel Xeon Scalable"),
	"i3en.3xlarge":  specs(3.1, 12, "3.1 GHz Intel Xeon Scalable"),
	"i3en.6xlarge":  specs(3.1, 24, "3.1 GHz Intel Xeon Scalable"),
	"i3en.12xlarge": specs(3.1, 48, "3.1 GHz Intel Xeon Scalable"),
	"i3en.24xlarge": specs(3.1, 96, "3.1 GHz Intel Xeon Scalable"),
	"i3en.metal":    specs(3.1, 96, "3.1 GHz Intel Xeon Scalable"),

	// d2
	"d2.xlarge":  specs(2.4, 4, "2.4 GHz Intel Xeon E5-2676 v3"),
	"d2.2xlarge": specs(2.4, 8, "2.4 GHz Intel Xeon E5-2676 v3"),
	"d2.4xlarge": specs(2.4, 16, "2.4 GHz Intel Xeon E5-2676 v3"),
	"d2.8xlarge": specs(2.4, 36, "2.4 GHz Intel Xeon E5-2676 v3"),

	// h1
	"h1.2xlarge":  specs(2.3, 8, "2.3 GHz Intel Xeon E5 2686 v4"),
	"h1.4xlarge":  specs(2.3, 16, "2.3 GHz Intel Xeon E5 2686 v4"),
	"h1.8xlarge":  specs(2.3, 32, "2.3 GHz Intel Xeon E5 2686 v4"),
	"h1.16xlarge": specs(2.3, 64, "2.3 GHz Intel Xeon E5 2686 v4"),
}

// EnvAWSFingerprint is used to fingerprint AWS metadata
type EnvAWSFingerprint struct {
	StaticFingerprinter

	// endpoint for EC2 metadata as expected by AWS SDK
	endpoint string

	logger log.Logger
}

// NewEnvAWSFingerprint is used to create a fingerprint from AWS metadata
func NewEnvAWSFingerprint(logger log.Logger) Fingerprint {
	f := &EnvAWSFingerprint{
		logger:   logger.Named("env_aws"),
		endpoint: strings.TrimSuffix(os.Getenv("AWS_ENV_URL"), "/meta-data/"),
	}
	return f
}

func (f *EnvAWSFingerprint) Fingerprint(request *FingerprintRequest, response *FingerprintResponse) error {
	cfg := request.Config

	timeout := AwsMetadataTimeout

	// Check if we should tighten the timeout
	if cfg.ReadBoolDefault(TightenNetworkTimeoutsConfig, false) {
		timeout = 1 * time.Millisecond
	}

	ec2meta, err := ec2MetaClient(f.endpoint, timeout)
	if err != nil {
		return fmt.Errorf("failed to setup ec2Metadata client: %v", err)
	}

	if !isAWS(ec2meta) {
		return nil
	}

	// Keys and whether they should be namespaced as unique. Any key whose value
	// uniquely identifies a node, such as ip, should be marked as unique. When
	// marked as unique, the key isn't included in the computed node class.
	keys := map[string]bool{
		"ami-id":                      false,
		"hostname":                    true,
		"instance-id":                 true,
		"instance-type":               false,
		"local-hostname":              true,
		"local-ipv4":                  true,
		"public-hostname":             true,
		"public-ipv4":                 true,
		"placement/availability-zone": false,
	}

	for k, unique := range keys {
		resp, err := ec2meta.GetMetadata(k)
		v := strings.TrimSpace(resp)
		if v == "" {
			f.logger.Debug("read an empty value", "attribute", k)
			continue
		} else if awsErr, ok := err.(awserr.RequestFailure); ok {
			f.logger.Debug("could not read attribute value", "attribute", k, "error", awsErr)
			continue
		} else if awsErr, ok := err.(awserr.Error); ok {
			// if it's a URL error, assume we're not in an AWS environment
			// TODO: better way to detect AWS? Check xen virtualization?
			if _, ok := awsErr.OrigErr().(*url.Error); ok {
				return nil
			}

			// not sure what other errors it would return
			return err
		}

		// assume we want blank entries
		key := "platform.aws." + strings.Replace(k, "/", ".", -1)
		if unique {
			key = structs.UniqueNamespace(key)
		}

		response.AddAttribute(key, v)
	}

	// accumulate resource information, then assign to response
	var resources *structs.Resources
	var nodeResources *structs.NodeResources

	// copy over network specific information
	if val, ok := response.Attributes["unique.platform.aws.local-ipv4"]; ok && val != "" {
		response.AddAttribute("unique.network.ip-address", val)
		nodeResources = new(structs.NodeResources)
		nodeResources.Networks = []*structs.NetworkResource{
			{
				Mode:   "host",
				Device: "eth0",
				IP:     val,
				CIDR:   val + "/32",
				MBits:  f.throughput(request, ec2meta, val),
			},
		}
	}

	// copy over CPU speed information
	if specs := f.lookupCPU(ec2meta); specs != nil {
		response.AddAttribute("cpu.modelname", specs.model)
		response.AddAttribute("cpu.frequency", fmt.Sprintf("%.0f", specs.mhz))
		response.AddAttribute("cpu.numcores", fmt.Sprintf("%d", specs.cores))
		f.logger.Debug("lookup ec2 cpu", "cores", specs.cores, "MHz", log.Fmt("%.0f", specs.mhz), "model", specs.model)

		if ticks := specs.ticks(); request.Config.CpuCompute <= 0 {
			response.AddAttribute("cpu.totalcompute", fmt.Sprintf("%d", ticks))
			f.logger.Debug("setting ec2 cpu ticks", "ticks", ticks)
			resources = new(structs.Resources)
			resources.CPU = ticks
			if nodeResources == nil {
				nodeResources = new(structs.NodeResources)
			}
			nodeResources.Cpu = structs.NodeCpuResources{CpuShares: int64(ticks)}
		}
	} else {
		f.logger.Warn("failed to find the cpu specification for this instance type")
	}

	response.Resources = resources
	response.NodeResources = nodeResources

	// populate Links
	response.AddLink("aws.ec2", fmt.Sprintf("%s.%s",
		response.Attributes["platform.aws.placement.availability-zone"],
		response.Attributes["unique.platform.aws.instance-id"]))
	response.Detected = true

	return nil
}

func (f *EnvAWSFingerprint) instanceType(ec2meta *ec2metadata.EC2Metadata) (string, error) {
	response, err := ec2meta.GetMetadata("instance-type")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(response), nil
}

func (f *EnvAWSFingerprint) lookupCPU(ec2meta *ec2metadata.EC2Metadata) *ec2Specs {
	instanceType, err := f.instanceType(ec2meta)
	if err != nil {
		f.logger.Warn("failed to read EC2 metadata instance-type", "error", err)
		return nil
	}
	for iType, specs := range ec2ProcSpeedTable {
		if strings.EqualFold(iType, instanceType) {
			return &specs
		}
	}
	return nil
}

func (f *EnvAWSFingerprint) throughput(request *FingerprintRequest, ec2meta *ec2metadata.EC2Metadata, ip string) int {
	throughput := request.Config.NetworkSpeed
	if throughput != 0 {
		return throughput
	}

	throughput = f.linkSpeed(ec2meta)
	if throughput != 0 {
		return throughput
	}

	if request.Node.Resources != nil && len(request.Node.Resources.Networks) > 0 {
		for _, n := range request.Node.Resources.Networks {
			if n.IP == ip {
				return n.MBits
			}
		}
	}

	return defaultNetworkSpeed
}

// EnvAWSFingerprint uses lookup table to approximate network speeds
func (f *EnvAWSFingerprint) linkSpeed(ec2meta *ec2metadata.EC2Metadata) int {
	instanceType, err := f.instanceType(ec2meta)
	if err != nil {
		f.logger.Error("error reading instance-type", "error", err)
		return 0
	}

	netSpeed := 0
	for reg, speed := range ec2NetSpeedTable {
		if reg.MatchString(instanceType) {
			netSpeed = speed
			break
		}
	}

	return netSpeed
}

func ec2MetaClient(endpoint string, timeout time.Duration) (*ec2metadata.EC2Metadata, error) {
	client := &http.Client{
		Timeout:   timeout,
		Transport: cleanhttp.DefaultTransport(),
	}

	c := aws.NewConfig().WithHTTPClient(client).WithMaxRetries(0)
	if endpoint != "" {
		c = c.WithEndpoint(endpoint)
	}

	sess, err := session.NewSession(c)
	if err != nil {
		return nil, err
	}
	return ec2metadata.New(sess, c), nil
}

func isAWS(ec2meta *ec2metadata.EC2Metadata) bool {
	v, err := ec2meta.GetMetadata("ami-id")
	v = strings.TrimSpace(v)
	return err == nil && v != ""
}
