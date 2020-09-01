module github.com/hashicorp/nomad

go 1.14

replace (
	github.com/Microsoft/go-winio => github.com/endocrimes/go-winio v0.4.13-0.20190628114223-fb47a8b41948
	github.com/NYTimes/gziphandler => github.com/NYTimes/gziphandler v1.0.0
	github.com/apparentlymart/go-textseg/v12 => github.com/apparentlymart/go-textseg/v12 v12.0.0
	github.com/godbus/dbus => github.com/godbus/dbus v5.0.1+incompatible
	github.com/golang/protobuf => github.com/golang/protobuf v1.3.4
	github.com/hashicorp/nomad/api => ./api
	github.com/kr/pty => github.com/kr/pty v1.1.5
	github.com/shirou/gopsutil => github.com/hashicorp/gopsutil v2.18.13-0.20200531184148-5aca383d4f9d+incompatible
)

require (
	cloud.google.com/go/storage v1.0.0 // indirect
	github.com/Azure/azure-sdk-for-go v29.0.0+incompatible // indirect
	github.com/Azure/go-autorest/autorest v0.11.4 // indirect
	github.com/Azure/go-autorest/autorest/azure/auth v0.5.1 // indirect
	github.com/Azure/go-autorest/autorest/to v0.4.0 // indirect
	github.com/Azure/go-autorest/autorest/validation v0.3.0 // indirect
	github.com/LK4D4/joincontext v0.0.0-20171026170139-1724345da6d5
	github.com/Microsoft/go-winio v0.4.15-0.20200113171025-3fe6c5262873
	github.com/Microsoft/hcsshim v0.8.8-0.20200312192636-fd0797d766b1 // indirect
	github.com/NVIDIA/gpu-monitoring-tools v0.0.0-20180829222009-86f2a9fac6c5
	github.com/NYTimes/gziphandler v1.0.1
	github.com/armon/circbuf v0.0.0-20150827004946-bbbad097214e
	github.com/armon/go-metrics v0.3.4
	github.com/aws/aws-sdk-go v1.25.41
	github.com/boltdb/bolt v1.3.1
	github.com/container-storage-interface/spec v1.2.0-rc1.0.20191021210849-a33ece0a8a9f
	github.com/containerd/go-cni v0.0.0-20190904155053-d20b7eebc7ee
	github.com/containernetworking/cni v0.7.2-0.20190612152420-dc953e2fd91f
	github.com/containernetworking/plugins v0.7.3-0.20190501191748-2d6d46d308b2
	github.com/coreos/go-iptables v0.4.3-0.20190724151750-969b135e941d
	github.com/coreos/go-semver v0.3.0
	github.com/cyphar/filepath-securejoin v0.2.3-0.20190205144030-7efe413b52e1 // indirect
	github.com/docker/cli v0.0.0-20200303215952-eb310fca4956
	github.com/docker/distribution v2.7.1+incompatible
	github.com/docker/docker v17.12.0-ce-rc1.0.20200330121334-7f8b4b621b5d+incompatible
	github.com/docker/docker-credential-helpers v0.6.2-0.20180719074751-73e5f5dbfea3 // indirect
	github.com/docker/go-units v0.4.0
	github.com/docker/libnetwork v0.8.0-dev.2.0.20200612180813-9e99af28df21
	github.com/dustin/go-humanize v1.0.0
	github.com/elazarl/go-bindata-assetfs v1.0.1-0.20200509193318-234c15e7648f
	github.com/fatih/color v1.9.0
	github.com/fsnotify/fsnotify v1.4.9 // indirect
	github.com/fsouza/go-dockerclient v1.6.5
	github.com/golang/protobuf v1.4.2
	github.com/golang/snappy v0.0.1
	github.com/google/go-cmp v0.4.0
	github.com/gorilla/websocket v1.4.2
	github.com/grpc-ecosystem/go-grpc-middleware v1.2.1-0.20200228141219-3ce3d519df39
	github.com/hashicorp/consul v1.7.7
	github.com/hashicorp/consul-template v0.24.1
	github.com/hashicorp/consul/api v1.6.0
	github.com/hashicorp/consul/sdk v0.6.0
	github.com/hashicorp/cronexpr v1.1.1
	github.com/hashicorp/go-checkpoint v0.0.0-20171009173528-1545e56e46de
	github.com/hashicorp/go-cleanhttp v0.5.1
	github.com/hashicorp/go-connlimit v0.2.0
	github.com/hashicorp/go-discover v0.0.0-20200108194735-7698de1390a1
	github.com/hashicorp/go-envparse v0.0.0-20180119215841-310ca1881b22
	github.com/hashicorp/go-getter v1.3.1-0.20190822194507-f5101da01173
	github.com/hashicorp/go-hclog v0.12.0
	github.com/hashicorp/go-immutable-radix v1.2.0
	github.com/hashicorp/go-memdb v1.2.1
	github.com/hashicorp/go-msgpack v1.1.5
	github.com/hashicorp/go-multierror v1.1.0
	github.com/hashicorp/go-plugin v1.0.2-0.20191004171845-809113480b55
	github.com/hashicorp/go-sockaddr v1.0.2
	github.com/hashicorp/go-syslog v1.0.0
	github.com/hashicorp/go-uuid v1.0.1
	github.com/hashicorp/go-version v1.2.1-0.20191009193637-2046c9d0f0b0
	github.com/hashicorp/golang-lru v0.5.4
	github.com/hashicorp/hcl v1.0.1-0.20191016231534-914dc3f8dd7c
	github.com/hashicorp/hcl/v2 v2.5.1
	github.com/hashicorp/logutils v1.0.0
	github.com/hashicorp/memberlist v0.2.2
	github.com/hashicorp/net-rpc-msgpackrpc v0.0.0-20151116020338-a14192a58a69
	github.com/hashicorp/nomad/api v0.0.0-20200529203653-c4416b26d3eb
	github.com/hashicorp/raft v1.1.3-0.20200211192230-365023de17e6
	github.com/hashicorp/raft-boltdb v0.0.0-20171010151810-6e5ba93211ea
	github.com/hashicorp/serf v0.9.3
	github.com/hashicorp/vault/api v1.0.5-0.20190730042357-746c0b111519
	github.com/hashicorp/vault/sdk v0.1.14-0.20190730042320-0dc007d98cc8
	github.com/hashicorp/yamux v0.0.0-20181012175058-2f1d1f20f75d
	github.com/hpcloud/tail v1.0.1-0.20170814160653-37f427138745
	github.com/ishidawataru/sctp v0.0.0-20191218070446-00ab2ac2db07 // indirect
	github.com/joyent/triton-go v0.0.0-20190112182421-51ffac552869 // indirect
	github.com/kr/pretty v0.2.0
	github.com/kr/pty v1.1.5
	github.com/kr/text v0.2.0
	github.com/mattn/go-colorable v0.1.6
	github.com/mitchellh/cli v1.1.0
	github.com/mitchellh/colorstring v0.0.0-20150917214807-8631ce90f286
	github.com/mitchellh/copystructure v1.0.0
	github.com/mitchellh/go-ps v0.0.0-20190716172923-621e5597135b
	github.com/mitchellh/go-testing-interface v1.0.3
	github.com/mitchellh/hashstructure v1.0.0
	github.com/mitchellh/mapstructure v1.3.1
	github.com/niemeyer/pretty v0.0.0-20200227124842-a10e7caefd8e // indirect
	github.com/oklog/run v1.0.1-0.20180308005104-6934b124db28 // indirect
	github.com/onsi/ginkgo v1.12.0
	github.com/onsi/gomega v1.9.0
	github.com/opencontainers/runc v1.0.0-rc92
	github.com/pkg/errors v0.9.1
	github.com/posener/complete v1.2.3
	github.com/prometheus/client_golang v1.4.0
	github.com/prometheus/common v0.9.1
	github.com/rs/cors v1.7.0
	github.com/ryanuber/columnize v2.1.1-0.20170703205827-abc90934186a+incompatible
	github.com/ryanuber/go-glob v1.0.0
	github.com/sean-/seed v0.0.0-20170313163322-e2103e2c3529
	github.com/seccomp/libseccomp-golang v0.9.2-0.20200314001724-bdab42bd5128 // indirect
	github.com/shirou/gopsutil v2.20.2+incompatible
	github.com/skratchdot/open-golang v0.0.0-20160302144031-75fb7ed4208c
	github.com/stretchr/objx v0.2.0 // indirect
	github.com/stretchr/testify v1.5.1
	github.com/syndtr/gocapability v0.0.0-20180916011248-d98352740cb2
	github.com/zclconf/go-cty v1.4.1
	go.opencensus.io v0.22.1-0.20190713072201-b4a14686f0a9 // indirect
	golang.org/x/crypto v0.0.0-20200622213623-75b288015ac9
	golang.org/x/exp v0.0.0-20191030013958-a1ab85dbe136 // indirect
	golang.org/x/lint v0.0.0-20191125180803-fdd1cda4f05f // indirect
	golang.org/x/mod v0.3.0 // indirect
	golang.org/x/net v0.0.0-20200324143707-d3edc9973b7e
	golang.org/x/oauth2 v0.0.0-20200107190931-bf48bf16ab8d // indirect
	golang.org/x/sync v0.0.0-20200317015054-43a5402ce75a
	golang.org/x/sys v0.0.0-20200728102440-3e129f6d46b1
	golang.org/x/text v0.3.3-0.20200306154105-06d492aade88 // indirect
	golang.org/x/time v0.0.0-20191024005414-555d28b269f0
	golang.org/x/tools v0.0.0-20200522201501-cb1345f3a375 // indirect
	google.golang.org/api v0.13.0 // indirect
	google.golang.org/genproto v0.0.0-20200302123026-7795fca6ccb1 // indirect
	google.golang.org/grpc v1.27.1
	gopkg.in/check.v1 v1.0.0-20200227125254-8fa46927fb4f // indirect
	gopkg.in/square/go-jose.v2 v2.4.1 // indirect
	gopkg.in/tomb.v1 v1.0.0-20141024135613-dd632973f1e7
	gopkg.in/tomb.v2 v2.0.0-20140626144623-14b3d72120e8
	gopkg.in/yaml.v2 v2.3.0 // indirect
	gotest.tools/v3 v3.0.2 // indirect
	honnef.co/go/tools v0.0.1-2020.1.4 // indirect
)
