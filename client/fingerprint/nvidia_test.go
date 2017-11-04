package fingerprint

import (
	"testing"
)

var output = `index, driver_version, name, uuid, memory.total [MiB]
0, 384.81, Tesla M40, GPU-491d88dd-da82-89f0-df6a-0ba07f9a08a9, 11443 MiB
1, 384.81, Tesla M40, GPU-7e6521dd-9383-aeb6-cadd-fdd2b2c9c358, 11443 MiB
2, 384.81, Tesla M40, GPU-852f5619-64b5-d6ab-12ab-4bd4102b9faf, 11443 MiB
3, 384.81, Tesla M40, GPU-69c95d23-68c3-80ad-3f04-da189ca227f6, 11443 MiB
4, 384.81, Tesla M40, GPU-a563104b-2b75-e663-448c-0d51c4ab5c1b, 11443 MiB
5, 384.81, Tesla M40, GPU-3bb6b279-7a9c-aafa-f537-686161f6f6be, 11443 MiB
6, 384.81, Tesla M40, GPU-08f53071-b844-6983-970c-3fa46931e631, 11443 MiB
7, 384.81, Tesla M40, GPU-70b656d4-07f0-bd16-3ae4-bf9305f34f5d, 11443 MiB
`

func TestParseOutput(t *testing.T) {
	f := &NvidiaGPUFingerprint{logger: testLogger()}
	gpus, err := f.parseOutput([]byte(output))
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(gpus) != 8 {
		t.Fatalf("gpus: %v", gpus)
	}
}
