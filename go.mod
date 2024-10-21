module github.com/hashicorp/nomad

go 1.23

// Pinned dependencies are noted in github.com/hashicorp/nomad/issues/11826.
replace (
	github.com/Microsoft/go-winio => github.com/endocrimes/go-winio v0.4.13-0.20190628114223-fb47a8b41948
	github.com/armon/go-metrics => github.com/armon/go-metrics v0.0.0-20230509193637-d9ca9af9f1f9
	github.com/hashicorp/hcl => github.com/hashicorp/hcl v1.0.1-0.20201016140508-a07e7d50bbee
)

// Nomad is built using the current source of the API module.
replace github.com/hashicorp/nomad/api => ./api

require (
	github.com/LK4D4/joincontext v0.0.0-20171026170139-1724345da6d5
	github.com/Masterminds/sprig/v3 v3.2.3
	github.com/Microsoft/go-winio v0.6.1
	github.com/armon/circbuf v0.0.0-20150827004946-bbbad097214e
	github.com/armon/go-metrics v0.5.3
	github.com/aws/aws-sdk-go v1.44.210
	github.com/brianvoe/gofakeit/v6 v6.20.1
	github.com/container-storage-interface/spec v1.10.0
	github.com/containerd/go-cni v1.1.9
	github.com/containernetworking/cni v1.2.3
	github.com/coreos/go-iptables v0.6.0
	github.com/creack/pty v1.1.23
	github.com/distribution/reference v0.5.0
	github.com/docker/cli v24.0.6+incompatible
	github.com/docker/docker v27.1.1+incompatible
	github.com/docker/go-connections v0.4.0
	github.com/docker/go-units v0.5.0
	github.com/dustin/go-humanize v1.0.1
	github.com/elazarl/go-bindata-assetfs v1.0.1
	github.com/fatih/color v1.17.0
	github.com/go-jose/go-jose/v3 v3.0.3
	github.com/golang-jwt/jwt/v5 v5.2.1
	github.com/golang/protobuf v1.5.4
	github.com/golang/snappy v0.0.4
	github.com/google/go-cmp v0.6.0
	github.com/gorilla/handlers v1.5.2
	github.com/gorilla/websocket v1.5.3
	github.com/gosuri/uilive v0.0.4
	github.com/grpc-ecosystem/go-grpc-middleware v1.4.0
	github.com/hashicorp/cap v0.6.0
	github.com/hashicorp/consul-template v0.39.0
	github.com/hashicorp/consul/api v1.29.1
	github.com/hashicorp/consul/sdk v0.16.1
	github.com/hashicorp/cronexpr v1.1.2
	github.com/hashicorp/go-bexpr v0.1.13
	github.com/hashicorp/go-checkpoint v0.5.0
	github.com/hashicorp/go-cleanhttp v0.5.2
	github.com/hashicorp/go-connlimit v0.3.0
	github.com/hashicorp/go-cty-funcs v0.0.0-20200930094925-2721b1e36840
	github.com/hashicorp/go-discover v0.0.0-20220621183603-a413e131e836
	github.com/hashicorp/go-envparse v0.1.0
	github.com/hashicorp/go-getter v1.7.5
	github.com/hashicorp/go-hclog v1.6.3
	github.com/hashicorp/go-immutable-radix/v2 v2.1.0
	github.com/hashicorp/go-kms-wrapping/v2 v2.0.16
	github.com/hashicorp/go-kms-wrapping/wrappers/awskms/v2 v2.0.9
	github.com/hashicorp/go-kms-wrapping/wrappers/azurekeyvault/v2 v2.0.11
	github.com/hashicorp/go-kms-wrapping/wrappers/gcpckms/v2 v2.0.12
	github.com/hashicorp/go-kms-wrapping/wrappers/transit/v2 v2.0.12
	github.com/hashicorp/go-memdb v1.3.4
	github.com/hashicorp/go-msgpack/v2 v2.1.2
	github.com/hashicorp/go-multierror v1.1.1
	github.com/hashicorp/go-netaddrs v0.1.0
	github.com/hashicorp/go-plugin v1.6.2
	github.com/hashicorp/go-secure-stdlib/listenerutil v0.1.9
	github.com/hashicorp/go-secure-stdlib/strutil v0.1.2
	github.com/hashicorp/go-set/v3 v3.0.0-alpha.1
	github.com/hashicorp/go-sockaddr v1.0.6
	github.com/hashicorp/go-syslog v1.0.0
	github.com/hashicorp/go-uuid v1.0.3
	github.com/hashicorp/go-version v1.7.0
	github.com/hashicorp/golang-lru/v2 v2.0.1
	github.com/hashicorp/hcl v1.0.1-vault-3
	github.com/hashicorp/hcl/v2 v2.20.2-0.20240517235513-55d9c02d147d
	github.com/hashicorp/hil v0.0.0-20210521165536-27a72121fd40
	github.com/hashicorp/logutils v1.0.0
	github.com/hashicorp/memberlist v0.5.1
	github.com/hashicorp/net-rpc-msgpackrpc/v2 v2.0.0
	github.com/hashicorp/nomad/api v0.0.0-20230103221135-ce00d683f9be
	github.com/hashicorp/raft v1.7.1
	github.com/hashicorp/raft-autopilot v0.1.6
	github.com/hashicorp/raft-boltdb/v2 v2.3.0
	github.com/hashicorp/serf v0.10.2-0.20240320153621-5d32001edfaa
	github.com/hashicorp/vault/api v1.10.0
	github.com/hashicorp/yamux v0.1.2
	github.com/hpcloud/tail v1.0.1-0.20170814160653-37f427138745
	github.com/klauspost/cpuid/v2 v2.2.8
	github.com/kr/pretty v0.3.1
	github.com/kr/text v0.2.0
	github.com/mattn/go-colorable v0.1.13
	github.com/miekg/dns v1.1.56
	github.com/mitchellh/cli v1.1.5
	github.com/mitchellh/colorstring v0.0.0-20150917214807-8631ce90f286
	github.com/mitchellh/copystructure v1.2.0
	github.com/mitchellh/go-glint v0.0.0-20210722152315-6515ceb4a127
	github.com/mitchellh/go-homedir v1.1.0
	github.com/mitchellh/go-ps v1.0.0
	github.com/mitchellh/go-testing-interface v1.14.2-0.20210821155943-2d9075ca8770
	github.com/mitchellh/hashstructure v1.1.0
	github.com/mitchellh/mapstructure v1.5.0
	github.com/mitchellh/pointerstructure v1.2.1
	github.com/mitchellh/reflectwalk v1.0.2
	github.com/moby/sys/mount v0.3.3
	github.com/moby/sys/mountinfo v0.7.1
	github.com/moby/term v0.5.0
	github.com/muesli/reflow v0.3.0
	github.com/opencontainers/image-spec v1.1.0
	github.com/opencontainers/runc v1.1.14
	github.com/opencontainers/runtime-spec v1.2.0
	github.com/posener/complete v1.2.3
	github.com/prometheus/client_golang v1.18.0
	github.com/prometheus/common v0.45.0
	github.com/rs/cors v1.8.3
	github.com/ryanuber/columnize v2.1.2+incompatible
	github.com/ryanuber/go-glob v1.0.0
	github.com/shirou/gopsutil/v3 v3.24.5
	github.com/shoenig/go-landlock v1.2.1
	github.com/shoenig/go-m1cpu v0.1.6
	github.com/shoenig/test v1.11.0
	github.com/stretchr/testify v1.9.0
	github.com/syndtr/gocapability v0.0.0-20200815063812-42c35b437635
	github.com/zclconf/go-cty v1.13.0
	github.com/zclconf/go-cty-yaml v1.0.3
	go.etcd.io/bbolt v1.3.9
	go.uber.org/goleak v1.2.1
	golang.org/x/crypto v0.27.0
	golang.org/x/mod v0.21.0
	golang.org/x/sync v0.8.0
	golang.org/x/sys v0.25.0
	golang.org/x/time v0.3.0
	google.golang.org/grpc v1.59.0
	google.golang.org/protobuf v1.34.2
	gopkg.in/tomb.v1 v1.0.0-20141024135613-dd632973f1e7
	gopkg.in/tomb.v2 v2.0.0-20140626144623-14b3d72120e8
	oss.indeed.com/go/libtime v1.6.0
)

require (
	cloud.google.com/go v0.110.8 // indirect
	cloud.google.com/go/compute v1.23.0 // indirect
	cloud.google.com/go/compute/metadata v0.2.3 // indirect
	cloud.google.com/go/iam v1.1.2 // indirect
	cloud.google.com/go/kms v1.15.2 // indirect
	cloud.google.com/go/storage v1.30.1 // indirect
	dario.cat/mergo v1.0.0 // indirect
	github.com/Azure/azure-sdk-for-go v68.0.0+incompatible // indirect
	github.com/Azure/azure-sdk-for-go/sdk/azcore v1.11.1 // indirect
	github.com/Azure/azure-sdk-for-go/sdk/azidentity v1.7.0 // indirect
	github.com/Azure/azure-sdk-for-go/sdk/internal v1.8.0 // indirect
	github.com/Azure/azure-sdk-for-go/sdk/keyvault/azkeys v0.10.0 // indirect
	github.com/Azure/azure-sdk-for-go/sdk/keyvault/internal v0.7.1 // indirect
	github.com/Azure/go-ansiterm v0.0.0-20210617225240-d185dfc1b5a1 // indirect
	github.com/Azure/go-autorest v14.2.0+incompatible // indirect
	github.com/Azure/go-autorest/autorest v0.11.29 // indirect
	github.com/Azure/go-autorest/autorest/adal v0.9.22 // indirect
	github.com/Azure/go-autorest/autorest/azure/auth v0.5.1 // indirect
	github.com/Azure/go-autorest/autorest/azure/cli v0.4.0 // indirect
	github.com/Azure/go-autorest/autorest/date v0.3.0 // indirect
	github.com/Azure/go-autorest/autorest/to v0.4.0 // indirect
	github.com/Azure/go-autorest/autorest/validation v0.3.0 // indirect
	github.com/Azure/go-autorest/logger v0.2.1 // indirect
	github.com/Azure/go-autorest/tracing v0.6.0 // indirect
	github.com/AzureAD/microsoft-authentication-library-for-go v1.2.2 // indirect
	github.com/BurntSushi/toml v1.3.2 // indirect
	github.com/DataDog/datadog-go v3.2.0+incompatible // indirect
	github.com/Masterminds/goutils v1.1.1 // indirect
	github.com/Masterminds/semver/v3 v3.2.0 // indirect
	github.com/VividCortex/ewma v1.1.1 // indirect
	github.com/agext/levenshtein v1.2.1 // indirect
	github.com/apparentlymart/go-cidr v1.0.1 // indirect
	github.com/apparentlymart/go-textseg/v13 v13.0.0 // indirect
	github.com/apparentlymart/go-textseg/v15 v15.0.0 // indirect
	github.com/armon/go-radix v1.0.0 // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/bgentry/go-netrc v0.0.0-20140422174119-9fd32a8b3d3d // indirect
	github.com/bgentry/speakeasy v0.1.0 // indirect
	github.com/bmatcuk/doublestar v1.1.5 // indirect
	github.com/boltdb/bolt v1.3.1 // indirect
	github.com/cenkalti/backoff/v3 v3.2.2 // indirect
	github.com/cespare/xxhash/v2 v2.2.0 // indirect
	github.com/checkpoint-restore/go-criu/v5 v5.3.0 // indirect
	github.com/cheggaaa/pb/v3 v3.0.5 // indirect
	github.com/cilium/ebpf v0.7.0 // indirect
	github.com/circonus-labs/circonus-gometrics v2.3.1+incompatible // indirect
	github.com/circonus-labs/circonusllhist v0.1.3 // indirect
	github.com/containerd/console v1.0.4 // indirect
	github.com/containerd/log v0.1.0 // indirect
	github.com/coreos/go-oidc/v3 v3.10.0 // indirect
	github.com/coreos/go-systemd/v22 v22.5.0 // indirect
	github.com/cyphar/filepath-securejoin v0.2.5 // indirect
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/denverdino/aliyungo v0.0.0-20190125010748-a747050bb1ba // indirect
	github.com/digitalocean/godo v1.10.0 // indirect
	github.com/dimchansky/utfbom v1.1.0 // indirect
	github.com/docker/distribution v2.8.3+incompatible // indirect
	github.com/docker/docker-credential-helpers v0.7.0 // indirect
	github.com/docker/go-metrics v0.0.1 // indirect
	github.com/docker/libtrust v0.0.0-20160708172513-aabc10ec26b7 // indirect
	github.com/felixge/httpsnoop v1.0.4 // indirect
	github.com/fsnotify/fsnotify v1.6.0 // indirect
	github.com/go-jose/go-jose/v4 v4.0.1 // indirect
	github.com/go-logr/logr v1.4.2 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/go-ole/go-ole v1.2.6 // indirect
	github.com/godbus/dbus/v5 v5.1.0 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/gojuno/minimock/v3 v3.0.6 // indirect
	github.com/golang-jwt/jwt/v4 v4.5.0 // indirect
	github.com/golang/groupcache v0.0.0-20210331224755-41bb18bfe9da // indirect
	github.com/google/btree v1.1.2 // indirect
	github.com/google/go-querystring v0.0.0-20170111101155-53e6ce116135 // indirect
	github.com/google/s2a-go v0.1.4 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/googleapis/enterprise-certificate-proxy v0.2.4 // indirect
	github.com/googleapis/gax-go/v2 v2.12.0 // indirect
	github.com/gookit/color v1.3.1 // indirect
	github.com/gophercloud/gophercloud v0.1.0 // indirect
	github.com/gorilla/mux v1.8.1 // indirect
	github.com/hashicorp/errwrap v1.1.0 // indirect
	github.com/hashicorp/go-immutable-radix v1.3.1 // indirect
	github.com/hashicorp/go-msgpack v1.1.6-0.20240304204939-8824e8ccc35f // indirect
	github.com/hashicorp/go-retryablehttp v0.7.7 // indirect
	github.com/hashicorp/go-rootcerts v1.0.2 // indirect
	github.com/hashicorp/go-safetemp v1.0.0 // indirect
	github.com/hashicorp/go-secure-stdlib/awsutil v0.1.6 // indirect
	github.com/hashicorp/go-secure-stdlib/parseutil v0.1.8 // indirect
	github.com/hashicorp/go-secure-stdlib/reloadutil v0.1.1 // indirect
	github.com/hashicorp/go-secure-stdlib/tlsutil v0.1.3 // indirect
	github.com/hashicorp/go-set/v2 v2.1.0 // indirect
	github.com/hashicorp/golang-lru v1.0.2 // indirect
	github.com/hashicorp/mdns v1.0.4 // indirect
	github.com/hashicorp/vault/api/auth/kubernetes v0.5.0 // indirect
	github.com/hashicorp/vic v1.5.1-0.20190403131502-bbfe86ec9443 // indirect
	github.com/huandu/xstrings v1.4.0 // indirect
	github.com/imdario/mergo v0.3.13 // indirect
	github.com/ishidawataru/sctp v0.0.0-20191218070446-00ab2ac2db07 // indirect
	github.com/jefferai/isbadcipher v0.0.0-20190226160619-51d2077c035f // indirect
	github.com/jmespath/go-jmespath v0.4.0 // indirect
	github.com/joyent/triton-go v0.0.0-20190112182421-51ffac552869 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/klauspost/compress v1.16.0 // indirect
	github.com/kylelemons/godebug v1.1.0 // indirect
	github.com/linode/linodego v0.7.1 // indirect
	github.com/lufia/plan9stats v0.0.0-20211012122336-39d0f177ccd0 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/mattn/go-runewidth v0.0.12 // indirect
	github.com/matttproud/golang_protobuf_extensions/v2 v2.0.0 // indirect
	github.com/mitchellh/go-wordwrap v1.0.1 // indirect
	github.com/moby/docker-image-spec v1.3.1 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/morikuni/aec v1.0.0 // indirect
	github.com/mrunalp/fileutils v0.5.1 // indirect
	github.com/nicolai86/scaleway-sdk v1.10.2-0.20180628010248-798f60e20bb2 // indirect
	github.com/oklog/run v1.1.0 // indirect
	github.com/opencontainers/go-digest v1.0.0 // indirect
	github.com/opencontainers/selinux v1.11.0 // indirect
	github.com/packethost/packngo v0.1.1-0.20180711074735-b9cb5096f54c // indirect
	github.com/pkg/browser v0.0.0-20240102092130-5ac0b6a4141c // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	github.com/power-devops/perfstat v0.0.0-20210106213030-5aafc221ea8c // indirect
	github.com/prometheus/client_model v0.5.0 // indirect
	github.com/prometheus/procfs v0.12.0 // indirect
	github.com/renier/xmlrpc v0.0.0-20170708154548-ce4a1a486c03 // indirect
	github.com/rivo/uniseg v0.2.0 // indirect
	github.com/rogpeppe/go-internal v1.12.0 // indirect
	github.com/sean-/seed v0.0.0-20170313163322-e2103e2c3529 // indirect
	github.com/seccomp/libseccomp-golang v0.10.0 // indirect
	github.com/shopspring/decimal v1.3.1 // indirect
	github.com/sirupsen/logrus v1.9.3 // indirect
	github.com/softlayer/softlayer-go v0.0.0-20180806151055-260589d94c7d // indirect
	github.com/spf13/cast v1.5.0 // indirect
	github.com/stretchr/objx v0.5.2 // indirect
	github.com/tencentcloud/tencentcloud-sdk-go v1.0.162 // indirect
	github.com/tj/go-spin v1.1.0 // indirect
	github.com/tklauser/go-sysconf v0.3.12 // indirect
	github.com/tklauser/numcpus v0.6.1 // indirect
	github.com/tv42/httpunix v0.0.0-20150427012821-b75d8614f926 // indirect
	github.com/ulikunitz/xz v0.5.10 // indirect
	github.com/vishvananda/netlink v1.2.1-beta.2 // indirect
	github.com/vishvananda/netns v0.0.4 // indirect
	github.com/vmihailenco/msgpack/v5 v5.3.5 // indirect
	github.com/vmihailenco/tagparser/v2 v2.0.0 // indirect
	github.com/vmware/govmomi v0.18.0 // indirect
	github.com/yusufpapurcu/wmi v1.2.4 // indirect
	go.opencensus.io v0.24.0 // indirect
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.55.0 // indirect
	go.opentelemetry.io/otel v1.30.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp v1.3.0 // indirect
	go.opentelemetry.io/otel/metric v1.30.0 // indirect
	go.opentelemetry.io/otel/sdk v1.21.0 // indirect
	go.opentelemetry.io/otel/trace v1.30.0 // indirect
	golang.org/x/exp v0.0.0-20240613232115-7f521ea00fb8 // indirect
	golang.org/x/net v0.26.0 // indirect
	golang.org/x/oauth2 v0.18.0 // indirect
	golang.org/x/term v0.24.0 // indirect
	golang.org/x/text v0.18.0 // indirect
	golang.org/x/tools v0.22.0 // indirect
	golang.org/x/xerrors v0.0.0-20220907171357-04be3eba64a2 // indirect
	google.golang.org/api v0.128.0 // indirect
	google.golang.org/appengine v1.6.8 // indirect
	google.golang.org/genproto v0.0.0-20230920204549-e6e6cdab5c13 // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20230913181813-007df8e322eb // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20231002182017-d307bd883b97 // indirect
	gopkg.in/fsnotify.v1 v1.4.7 // indirect
	gopkg.in/resty.v1 v1.12.0 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
	gotest.tools/v3 v3.5.1 // indirect
	kernel.org/pub/linux/libs/security/libcap/psx v1.2.69 // indirect
)
