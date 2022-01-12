module github.com/hashicorp/nomad

go 1.17

// Pinned dependencies are noted in github.com/hashicorp/nomad/issues/11826
replace (
	github.com/Microsoft/go-winio => github.com/endocrimes/go-winio v0.4.13-0.20190628114223-fb47a8b41948
	github.com/NYTimes/gziphandler => github.com/NYTimes/gziphandler v1.0.0
	github.com/apparentlymart/go-textseg/v12 => github.com/apparentlymart/go-textseg/v12 v12.0.0
	github.com/hashicorp/go-discover => github.com/hashicorp/go-discover v0.0.0-20210818145131-c573d69da192
	github.com/hashicorp/hcl => github.com/hashicorp/hcl v1.0.1-0.20201016140508-a07e7d50bbee
	github.com/kr/pty => github.com/kr/pty v1.1.5
)

// Nomad is built using the current source of the API module
replace github.com/hashicorp/nomad/api => ./api

require (
	github.com/LK4D4/joincontext v0.0.0-20171026170139-1724345da6d5
	github.com/Microsoft/go-winio v0.4.15-0.20200113171025-3fe6c5262873
	github.com/NYTimes/gziphandler v1.0.1
	github.com/armon/circbuf v0.0.0-20150827004946-bbbad097214e
	github.com/armon/go-metrics v0.3.10
	github.com/aws/aws-sdk-go v1.42.6
	github.com/boltdb/bolt v1.3.1
	github.com/container-storage-interface/spec v1.4.0
	github.com/containerd/go-cni v0.0.0-20190904155053-d20b7eebc7ee
	github.com/containernetworking/cni v0.7.2-0.20190612152420-dc953e2fd91f
	github.com/containernetworking/plugins v0.7.3-0.20190501191748-2d6d46d308b2
	github.com/coreos/go-iptables v0.4.3-0.20190724151750-969b135e941d
	github.com/coreos/go-semver v0.3.0
	github.com/docker/cli v0.0.0-20200303215952-eb310fca4956
	github.com/docker/distribution v2.7.1+incompatible
	github.com/docker/docker v17.12.0-ce-rc1.0.20200330121334-7f8b4b621b5d+incompatible
	github.com/docker/go-units v0.4.0
	github.com/docker/libnetwork v0.8.0-dev.2.0.20200612180813-9e99af28df21
	github.com/dustin/go-humanize v1.0.0
	github.com/elazarl/go-bindata-assetfs v1.0.1-0.20200509193318-234c15e7648f
	github.com/fatih/color v1.13.0
	github.com/fsouza/go-dockerclient v1.6.5
	github.com/golang/protobuf v1.5.2
	github.com/golang/snappy v0.0.4
	github.com/google/go-cmp v0.5.6
	github.com/gorilla/websocket v1.4.2
	github.com/gosuri/uilive v0.0.4
	github.com/grpc-ecosystem/go-grpc-middleware v1.2.1-0.20200228141219-3ce3d519df39
	github.com/hashicorp/consul v1.7.8
	github.com/hashicorp/consul-template v0.25.2
	github.com/hashicorp/consul/api v1.9.1
	github.com/hashicorp/consul/sdk v0.8.0
	github.com/hashicorp/cronexpr v1.1.1
	github.com/hashicorp/go-checkpoint v0.0.0-20171009173528-1545e56e46de
	github.com/hashicorp/go-cleanhttp v0.5.2
	github.com/hashicorp/go-connlimit v0.3.0
	github.com/hashicorp/go-cty-funcs v0.0.0-20200930094925-2721b1e36840
	// NOTE: update the version for github.com/hashicorp/go-discover in the
	// `replace` block as well to prevent other dependencies from pulling older
	// versions.
	github.com/hashicorp/go-discover v0.0.0-20210818145131-c573d69da192
	github.com/hashicorp/go-envparse v0.0.0-20180119215841-310ca1881b22
	github.com/hashicorp/go-getter v1.5.10
	github.com/hashicorp/go-hclog v1.0.0
	github.com/hashicorp/go-immutable-radix v1.3.0
	github.com/hashicorp/go-memdb v1.3.2
	github.com/hashicorp/go-msgpack v1.1.5
	github.com/hashicorp/go-multierror v1.1.1
	github.com/hashicorp/go-plugin v1.4.3
	github.com/hashicorp/go-sockaddr v1.0.2
	github.com/hashicorp/go-syslog v1.0.0
	github.com/hashicorp/go-uuid v1.0.2
	github.com/hashicorp/go-version v1.3.0
	github.com/hashicorp/golang-lru v0.5.4
	github.com/hashicorp/hcl v1.0.1-0.20201016140508-a07e7d50bbee
	github.com/hashicorp/hcl/v2 v2.9.2-0.20210407182552-eb14f8319bdc
	github.com/hashicorp/logutils v1.0.0
	github.com/hashicorp/memberlist v0.2.2
	github.com/hashicorp/net-rpc-msgpackrpc v0.0.0-20151116020338-a14192a58a69
	github.com/hashicorp/nomad/api v0.0.0-20200529203653-c4416b26d3eb
	github.com/hashicorp/raft v1.1.3-0.20200211192230-365023de17e6
	github.com/hashicorp/raft-boltdb v0.0.0-20171010151810-6e5ba93211ea
	github.com/hashicorp/serf v0.9.5
	github.com/hashicorp/vault/api v1.0.5-0.20200805123347-1ef507638af6
	github.com/hashicorp/vault/sdk v0.2.0
	github.com/hashicorp/yamux v0.0.0-20181012175058-2f1d1f20f75d
	github.com/hpcloud/tail v1.0.1-0.20170814160653-37f427138745
	github.com/kr/pretty v0.3.0
	github.com/kr/pty v1.1.5
	github.com/kr/text v0.2.0
	github.com/mattn/go-colorable v0.1.9
	github.com/miekg/dns v1.1.26
	github.com/mitchellh/cli v1.1.0
	github.com/mitchellh/colorstring v0.0.0-20150917214807-8631ce90f286
	github.com/mitchellh/copystructure v1.1.1
	github.com/mitchellh/go-glint v0.0.0-20210722152315-6515ceb4a127
	github.com/mitchellh/go-ps v0.0.0-20190716172923-621e5597135b
	github.com/mitchellh/go-testing-interface v1.14.1
	github.com/mitchellh/hashstructure v1.0.0
	github.com/mitchellh/mapstructure v1.4.2
	github.com/mitchellh/reflectwalk v1.0.1
	github.com/opencontainers/runc v1.0.0-rc93
	github.com/opencontainers/runtime-spec v1.0.3-0.20200929063507-e6143ca7d51d
	github.com/pkg/errors v0.9.1
	github.com/posener/complete v1.2.3
	github.com/prometheus/client_golang v1.4.0
	github.com/prometheus/common v0.9.1
	github.com/rs/cors v1.8.0
	github.com/ryanuber/columnize v2.1.1-0.20170703205827-abc90934186a+incompatible
	github.com/ryanuber/go-glob v1.0.0
	github.com/sean-/seed v0.0.0-20170313163322-e2103e2c3529
	github.com/shirou/gopsutil/v3 v3.21.6-0.20210619153009-7ea8062810b6
	github.com/skratchdot/open-golang v0.0.0-20160302144031-75fb7ed4208c
	github.com/stretchr/testify v1.7.0
	github.com/syndtr/gocapability v0.0.0-20200815063812-42c35b437635
	github.com/zclconf/go-cty v1.8.0
	github.com/zclconf/go-cty-yaml v1.0.2
	go.uber.org/goleak v1.1.12
	golang.org/x/crypto v0.0.0-20210921155107-089bfa567519
	golang.org/x/net v0.0.0-20211108170745-6635138e15ea
	golang.org/x/sync v0.0.0-20210220032951-036812b2e83c
	golang.org/x/sys v0.0.0-20211109065445-02f5c0300f6e
	golang.org/x/time v0.0.0-20200630173020-3af7569d3a1e
	google.golang.org/grpc v1.42.0
	gopkg.in/tomb.v1 v1.0.0-20141024135613-dd632973f1e7
	gopkg.in/tomb.v2 v2.0.0-20140626144623-14b3d72120e8
)

require (
	cloud.google.com/go v0.97.0 // indirect
	cloud.google.com/go/storage v1.18.2 // indirect
	github.com/Azure/azure-sdk-for-go v44.0.0+incompatible // indirect
	github.com/Azure/go-ansiterm v0.0.0-20170929234023-d6e3b3328b78 // indirect
	github.com/Azure/go-autorest v14.2.0+incompatible // indirect
	github.com/Azure/go-autorest/autorest v0.11.4 // indirect
	github.com/Azure/go-autorest/autorest/adal v0.9.2 // indirect
	github.com/Azure/go-autorest/autorest/azure/auth v0.5.1 // indirect
	github.com/Azure/go-autorest/autorest/azure/cli v0.4.0 // indirect
	github.com/Azure/go-autorest/autorest/date v0.3.0 // indirect
	github.com/Azure/go-autorest/autorest/to v0.4.0 // indirect
	github.com/Azure/go-autorest/autorest/validation v0.3.0 // indirect
	github.com/Azure/go-autorest/logger v0.2.0 // indirect
	github.com/Azure/go-autorest/tracing v0.6.0 // indirect
	github.com/BurntSushi/toml v0.4.1 // indirect
	github.com/DataDog/datadog-go v3.2.0+incompatible // indirect
	github.com/Microsoft/hcsshim v0.8.9 // indirect
	github.com/StackExchange/wmi v0.0.0-20190523213315-cbe66965904d // indirect
	github.com/VividCortex/ewma v1.1.1 // indirect
	github.com/agext/levenshtein v1.2.1 // indirect
	github.com/apparentlymart/go-cidr v1.0.1 // indirect
	github.com/apparentlymart/go-textseg/v13 v13.0.0 // indirect
	github.com/armon/go-radix v1.0.0 // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/bgentry/go-netrc v0.0.0-20140422174119-9fd32a8b3d3d // indirect
	github.com/bgentry/speakeasy v0.1.0 // indirect
	github.com/bmatcuk/doublestar v1.1.5 // indirect
	github.com/census-instrumentation/opencensus-proto v0.3.0 // indirect
	github.com/cespare/xxhash/v2 v2.1.2 // indirect
	github.com/checkpoint-restore/go-criu/v4 v4.1.0 // indirect
	github.com/cheggaaa/pb/v3 v3.0.5 // indirect
	github.com/cilium/ebpf v0.2.0 // indirect
	github.com/circonus-labs/circonus-gometrics v2.3.1+incompatible // indirect
	github.com/circonus-labs/circonusllhist v0.1.3 // indirect
	github.com/cncf/udpa/go v0.0.0-20210930031921-04548b0d99d4 // indirect
	github.com/cncf/xds/go v0.0.0-20211011173535-cb28da3451f1 // indirect
	github.com/containerd/console v1.0.1 // indirect
	github.com/containerd/containerd v1.3.4 // indirect
	github.com/containerd/continuity v0.0.0-20200709052629-daa8e1ccc0bc // indirect
	github.com/coreos/go-systemd/v22 v22.1.0 // indirect
	github.com/cyphar/filepath-securejoin v0.2.3-0.20190205144030-7efe413b52e1 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/denverdino/aliyungo v0.0.0-20190125010748-a747050bb1ba // indirect
	github.com/dgrijalva/jwt-go v3.2.0+incompatible // indirect
	github.com/digitalocean/godo v1.10.0 // indirect
	github.com/dimchansky/utfbom v1.1.0 // indirect
	github.com/docker/docker-credential-helpers v0.6.2-0.20180719074751-73e5f5dbfea3 // indirect
	github.com/docker/go-connections v0.4.0 // indirect
	github.com/docker/go-metrics v0.0.1 // indirect
	github.com/docker/libtrust v0.0.0-20160708172513-aabc10ec26b7 // indirect
	github.com/envoyproxy/go-control-plane v0.10.0 // indirect
	github.com/envoyproxy/protoc-gen-validate v0.6.2 // indirect
	github.com/fsnotify/fsnotify v1.4.9 // indirect
	github.com/go-ole/go-ole v1.2.4 // indirect
	github.com/godbus/dbus/v5 v5.0.3 // indirect
	github.com/gogo/protobuf v1.3.1 // indirect
	github.com/golang/groupcache v0.0.0-20210331224755-41bb18bfe9da // indirect
	github.com/google/btree v1.0.0 // indirect
	github.com/google/go-querystring v0.0.0-20170111101155-53e6ce116135 // indirect
	github.com/google/uuid v1.1.2 // indirect
	github.com/googleapis/gax-go/v2 v2.1.1 // indirect
	github.com/gookit/color v1.3.1 // indirect
	github.com/gophercloud/gophercloud v0.1.0 // indirect
	github.com/gorilla/mux v1.7.4 // indirect
	github.com/hashicorp/errwrap v1.1.0 // indirect
	github.com/hashicorp/go-retryablehttp v0.6.7 // indirect
	github.com/hashicorp/go-rootcerts v1.0.2 // indirect
	github.com/hashicorp/go-safetemp v1.0.0 // indirect
	github.com/hashicorp/mdns v1.0.1 // indirect
	github.com/hashicorp/vic v1.5.1-0.20190403131502-bbfe86ec9443 // indirect
	github.com/ishidawataru/sctp v0.0.0-20191218070446-00ab2ac2db07 // indirect
	github.com/jmespath/go-jmespath v0.4.0 // indirect
	github.com/joyent/triton-go v0.0.0-20190112182421-51ffac552869 // indirect
	github.com/klauspost/compress v1.13.6 // indirect
	github.com/linode/linodego v0.7.1 // indirect
	github.com/mattn/go-isatty v0.0.14 // indirect
	github.com/mattn/go-runewidth v0.0.7 // indirect
	github.com/mattn/go-shellwords v1.0.10 // indirect
	github.com/matttproud/golang_protobuf_extensions v1.0.1 // indirect
	github.com/mitchellh/go-homedir v1.1.0 // indirect
	github.com/mitchellh/go-wordwrap v1.0.1 // indirect
	github.com/moby/sys/mountinfo v0.4.0 // indirect
	github.com/morikuni/aec v1.0.0 // indirect
	github.com/mrunalp/fileutils v0.5.0 // indirect
	github.com/nicolai86/scaleway-sdk v1.10.2-0.20180628010248-798f60e20bb2 // indirect
	github.com/niemeyer/pretty v0.0.0-20200227124842-a10e7caefd8e // indirect
	github.com/oklog/run v1.0.1-0.20180308005104-6934b124db28 // indirect
	github.com/onsi/gomega v1.9.0 // indirect
	github.com/opencontainers/go-digest v1.0.0 // indirect
	github.com/opencontainers/image-spec v1.0.1 // indirect
	github.com/opencontainers/selinux v1.8.0 // indirect
	github.com/packethost/packngo v0.1.1-0.20180711074735-b9cb5096f54c // indirect
	github.com/pierrec/lz4 v2.5.2+incompatible // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/prometheus/client_model v0.2.0 // indirect
	github.com/prometheus/procfs v0.0.8 // indirect
	github.com/renier/xmlrpc v0.0.0-20170708154548-ce4a1a486c03 // indirect
	github.com/rogpeppe/go-internal v1.6.1 // indirect
	github.com/seccomp/libseccomp-golang v0.9.2-0.20200314001724-bdab42bd5128 // indirect
	github.com/sirupsen/logrus v1.8.1 // indirect
	github.com/softlayer/softlayer-go v0.0.0-20180806151055-260589d94c7d // indirect
	github.com/stretchr/objx v0.2.0 // indirect
	github.com/tencentcloud/tencentcloud-sdk-go v1.0.162 // indirect
	github.com/tj/go-spin v1.1.0 // indirect
	github.com/tklauser/go-sysconf v0.3.6 // indirect
	github.com/tklauser/numcpus v0.2.2 // indirect
	github.com/tv42/httpunix v0.0.0-20150427012821-b75d8614f926 // indirect
	github.com/ulikunitz/xz v0.5.10 // indirect
	github.com/vishvananda/netlink v1.1.0 // indirect
	github.com/vishvananda/netns v0.0.0-20191106174202-0a2b9b5464df // indirect
	github.com/vmihailenco/msgpack/v4 v4.3.12 // indirect
	github.com/vmihailenco/tagparser v0.1.1 // indirect
	github.com/vmware/govmomi v0.18.0 // indirect
	github.com/willf/bitset v1.1.11 // indirect
	go.opencensus.io v0.23.0 // indirect
	golang.org/x/oauth2 v0.0.0-20211104180415-d3ed0bb246c8 // indirect
	golang.org/x/term v0.0.0-20201126162022-7de9c90e9dd1 // indirect
	golang.org/x/text v0.3.7 // indirect
	golang.org/x/tools v0.1.8-0.20211029000441-d6a9af8af023 // indirect
	golang.org/x/xerrors v0.0.0-20200804184101-5ec99f83aff1 // indirect
	google.golang.org/api v0.60.0 // indirect
	google.golang.org/appengine v1.6.7 // indirect
	google.golang.org/genproto v0.0.0-20211104193956-4c6863e31247 // indirect
	google.golang.org/protobuf v1.27.1 // indirect
	gopkg.in/check.v1 v1.0.0-20200227125254-8fa46927fb4f // indirect
	gopkg.in/fsnotify.v1 v1.4.7 // indirect
	gopkg.in/resty.v1 v1.12.0 // indirect
	gopkg.in/square/go-jose.v2 v2.5.1 // indirect
	gopkg.in/yaml.v2 v2.3.0 // indirect
	gopkg.in/yaml.v3 v3.0.0-20200313102051-9f266ea9e77c // indirect
)
