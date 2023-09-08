/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

export const JOB_JSON = {
  Shallow: false,
  CreateRecommendations: true,
  WithTaskServices: false,
  WithGroupServices: false,
  WithRescheduling: false,
  NoHostVolumes: false,
  NoFailedPlacements: false,
  FailedPlacements: false,
  NoDeployments: false,
  ActiveDeployment: false,
  NoActiveDeployment: false,
  CreateAllocations: true,
  ModifyIndex: 1201,
  CreateIndex: 0,
  Meta: null,
  ChildrenCount: 1,
  Datacenters: ['cl12'],
  Status: 'running',
  AllAtOnce: true,
  Priority: 73,
  Type: 'service',
  Region: 'global',
  ResourceSpec: null,
  GroupsCount: 2,
  SubmitTime: 1676365074506000000,
  Version: 1,
  ID: 'hdd-panel-0',
  Name: 'hdd-panel-0',
  Namespace: 'namespace-1',
  NamespaceID: 'namespace-1',
  TaskGroups: [
    {
      ResourceSpec: null,
      Shallow: false,
      CreateRecommendations: true,
      WithTaskServices: false,
      WithServices: false,
      WithRescheduling: false,
      CreateAllocations: true,
      Volumes: {
        mazie: {
          Name: 'mazie',
          Type: 'host',
          Source: 'claire',
          ReadOnly: false,
        },
        leora: {
          Name: 'leora',
          Type: 'host',
          Source: 'jamil',
          ReadOnly: false,
        },
      },
      WithScaling: false,
      EphemeralDisk: {
        Sticky: true,
        SizeMB: 5000,
        Migrate: false,
      },
      Count: 2,
      Name: 'pixel-g-0',
      ID: '1',
      Services: null,
      Tasks: [
        {
          TaskGroupID: '1',
          Lifecycle: null,
          OriginalResources: {
            Cpu: {
              CpuShares: 250,
            },
            Memory: {
              MemoryMB: 1024,
              MemoryMaxMB: 8192,
            },
            Disk: {
              DiskMB: 0,
            },
            Networks: [
              {
                Device: 'eth4',
                CIDR: '',
                IP: '116.206.32.192',
                MBits: 10,
                Mode: 'bridge',
                ReservedPorts: [],
                DynamicPorts: [
                  {
                    Label: 'sensor',
                    Value: 43123,
                    To: 29512,
                  },
                ],
              },
              {
                Device: 'eth3',
                CIDR: '',
                IP: '172.75.114.89',
                MBits: 10,
                Mode: 'host',
                ReservedPorts: [],
                DynamicPorts: [
                  {
                    Label: 'firewall',
                    Value: 43343,
                    To: 38426,
                  },
                  {
                    Label: 'pixel',
                    Value: 16314,
                    To: 13304,
                  },
                ],
              },
              {
                Device: 'eth5',
                CIDR: '',
                IP: '194.36.199.184',
                MBits: 10,
                Mode: 'host',
                ReservedPorts: [
                  {
                    Label: 'program',
                    Value: 48937,
                    To: 14717,
                  },
                ],
                DynamicPorts: [
                  {
                    Label: 'interface',
                    Value: 44940,
                    To: 22631,
                  },
                ],
              },
            ],
            Ports: [
              {
                Label: 'transmitter',
                Value: 40616,
                To: 38744,
                HostIP: 'c760:161f:5d3e:d49a:b973:1234:7835:07bd',
              },
            ],
          },
          Resources: {
            CPU: 250,
            MemoryMB: 1024,
            MemoryMaxMB: 8192,
            DiskMB: 0,
          },
          Driver: 'java',
          Name: 'task-hard-drive-0',
          JobID: '',
          VolumeMounts: [
            {
              Volume: 'leora',
              Destination: '/Andreanne_Gulgowski94/jordane/#442118',
              PropagationMode: '',
              ReadOnly: true,
            },
            {
              Volume: 'mazie',
              Destination: '/Nicholaus43/erna/#377517',
              PropagationMode: '',
              ReadOnly: true,
            },
          ],
          GroupNames: [],
          WithServices: false,
          CreateRecommendations: true,
          ID: '1',
          Services: [],
        },
        {
          TaskGroupID: '1',
          Lifecycle: {
            Hook: 'prestart',
            Sidecar: false,
          },
          OriginalResources: {
            Cpu: {
              CpuShares: 250,
            },
            Memory: {
              MemoryMB: 2048,
              MemoryMaxMB: 0,
            },
            Disk: {
              DiskMB: 0,
            },
            Networks: [
              {
                Device: 'eth0',
                CIDR: '',
                IP: '124.146.107.83',
                MBits: 10,
                Mode: 'bridge',
                ReservedPorts: [],
                DynamicPorts: [
                  {
                    Label: 'alarm',
                    Value: 6224,
                    To: 42238,
                  },
                  {
                    Label: 'transmitter',
                    Value: 37450,
                    To: 15365,
                  },
                ],
              },
              {
                Device: 'eth2',
                CIDR: '',
                IP: '175.247.90.51',
                MBits: 10,
                Mode: 'host',
                ReservedPorts: [
                  {
                    Label: 'bandwidth',
                    Value: 20203,
                    To: 15316,
                  },
                ],
                DynamicPorts: [
                  {
                    Label: 'circuit',
                    Value: 18394,
                    To: 37087,
                  },
                  {
                    Label: 'protocol',
                    Value: 22008,
                    To: 12761,
                  },
                ],
              },
              {
                Device: 'eth0',
                CIDR: '',
                IP: '21.58.85.37',
                MBits: 10,
                Mode: 'bridge',
                ReservedPorts: [],
                DynamicPorts: [
                  {
                    Label: 'circuit',
                    Value: 12116,
                    To: 53021,
                  },
                  {
                    Label: 'application',
                    Value: 43516,
                    To: 19386,
                  },
                ],
              },
            ],
            Ports: [
              {
                Label: 'bandwidth',
                Value: 50493,
                To: 58903,
                HostIP: '7534:4ef6:c704:0e86:643b:7311:2dab:b933',
              },
            ],
          },
          Resources: {
            CPU: 250,
            MemoryMB: 2048,
            MemoryMaxMB: 0,
            DiskMB: 0,
          },
          Driver: 'qemu',
          Name: 'task-transmitter-1',
          JobID: '',
          VolumeMounts: [
            {
              Volume: 'mazie',
              Destination: '/Sylvan79/marjorie/#407369',
              PropagationMode: '',
              ReadOnly: false,
            },
            {
              Volume: 'leora',
              Destination: '/Jeanie.Thiel75/ross/#365510',
              PropagationMode: '',
              ReadOnly: false,
            },
          ],
          GroupNames: [],
          WithServices: false,
          CreateRecommendations: true,
          ID: '2',
          Services: [],
        },
      ],
    },
    {
      ResourceSpec: null,
      Shallow: false,
      CreateRecommendations: true,
      WithTaskServices: false,
      WithServices: false,
      WithRescheduling: false,
      CreateAllocations: true,
      Volumes: {
        mazie: {
          Name: 'mazie',
          Type: 'host',
          Source: 'claire',
          ReadOnly: false,
        },
        leora: {
          Name: 'leora',
          Type: 'host',
          Source: 'jamil',
          ReadOnly: false,
        },
      },
      WithScaling: true,
      EphemeralDisk: {
        Sticky: false,
        SizeMB: 500,
        Migrate: false,
      },
      Count: 2,
      Name: 'protocol-g-1',
      ID: '2',
      Scaling: {
        Min: 1,
        Max: 5,
        Policy: false,
      },
      Services: null,
      Tasks: [
        {
          TaskGroupID: '2',
          Lifecycle: {
            Hook: 'prestart',
            Sidecar: true,
          },
          OriginalResources: {
            Cpu: {
              CpuShares: 4000,
            },
            Memory: {
              MemoryMB: 4096,
              MemoryMaxMB: 8192,
            },
            Disk: {
              DiskMB: 0,
            },
            Networks: [
              {
                Device: 'eth1',
                CIDR: '',
                IP: '203.214.83.7',
                MBits: 10,
                Mode: 'bridge',
                ReservedPorts: [],
                DynamicPorts: [
                  {
                    Label: 'bus',
                    Value: 58731,
                    To: 43657,
                  },
                  {
                    Label: 'firewall',
                    Value: 31480,
                    To: 57357,
                  },
                ],
              },
            ],
            Ports: [
              {
                Label: 'circuit',
                Value: 53773,
                To: 33492,
                HostIP: '55b9:739b:f8cb:591f:238c:6ec3:e925:defc',
              },
            ],
          },
          Resources: {
            CPU: 4000,
            MemoryMB: 4096,
            MemoryMaxMB: 8192,
            DiskMB: 0,
          },
          Driver: 'docker',
          Name: 'task-firewall-2',
          JobID: '',
          VolumeMounts: [
            {
              Volume: 'mazie',
              Destination: '/Wanda21/ronaldo/#315877',
              PropagationMode: '',
              ReadOnly: false,
            },
          ],
          GroupNames: [],
          WithServices: false,
          CreateRecommendations: true,
          ID: '3',
          Services: [],
        },
        {
          TaskGroupID: '2',
          Lifecycle: {
            Hook: 'poststart',
            Sidecar: false,
          },
          OriginalResources: {
            Cpu: {
              CpuShares: 2000,
            },
            Memory: {
              MemoryMB: 1024,
              MemoryMaxMB: 0,
            },
            Disk: {
              DiskMB: 0,
            },
            Networks: [
              {
                Device: 'eth1',
                CIDR: '',
                IP: '126.161.123.8',
                MBits: 10,
                Mode: 'bridge',
                ReservedPorts: [
                  {
                    Label: 'alarm',
                    Value: 52478,
                    To: 11878,
                  },
                ],
                DynamicPorts: [
                  {
                    Label: 'application',
                    Value: 5543,
                    To: 22670,
                  },
                  {
                    Label: 'port',
                    Value: 19767,
                    To: 17311,
                  },
                ],
              },
              {
                Device: 'eth0',
                CIDR: '',
                IP: '12.228.2.247',
                MBits: 10,
                Mode: 'bridge',
                ReservedPorts: [],
                DynamicPorts: [],
              },
              {
                Device: 'eth1',
                CIDR: '',
                IP: '71.92.163.164',
                MBits: 10,
                Mode: 'bridge',
                ReservedPorts: [],
                DynamicPorts: [],
              },
            ],
            Ports: [
              {
                Label: 'pixel',
                Value: 32161,
                To: 25929,
                HostIP: 'b778:a055:4422:e3ca:fd2d:e5e8:464d:8f2b',
              },
            ],
          },
          Resources: {
            CPU: 2000,
            MemoryMB: 1024,
            MemoryMaxMB: 0,
            DiskMB: 0,
          },
          Driver: 'qemu',
          Name: 'task-microchip-3',
          JobID: '',
          VolumeMounts: [
            {
              Volume: 'mazie',
              Destination: '/Vella.OReilly/dudley/#4b403a',
              PropagationMode: '',
              ReadOnly: true,
            },
            {
              Volume: 'leora',
              Destination: '/Tressa_Brown/ian/#6d666c',
              PropagationMode: '',
              ReadOnly: false,
            },
          ],
          GroupNames: [],
          WithServices: false,
          CreateRecommendations: true,
          ID: '4',
          Services: [],
        },
      ],
    },
  ],
  JobSummary: {
    GroupNames: ['pixel-g-0', 'protocol-g-1'],
    Summary: {
      'pixel-g-0': {
        Queued: 10,
        Complete: 10,
        Failed: 6,
        Running: 7,
        Starting: 4,
        Lost: 8,
        Unknown: 1,
      },
      'protocol-g-1': {
        Queued: 4,
        Complete: 7,
        Failed: 2,
        Running: 0,
        Starting: 7,
        Lost: 3,
        Unknown: 5,
      },
    },
    Namespace: 'namespace-1',
    ID: '1',
    JobID: 'hdd-panel-0',
  },
  Specification: {
    Definition:
      'job "docs" {\n  namespace = "madness"\n  group "example" {\n    task "server" {\n      service {\n        tags = ["leader", "mysql"]\n\n        port = "db"\n\n        meta {\n          meta = "for your service"\n        }\n\n        check {\n          type     = "tcp"\n          port     = "db"\n          interval = "10s"\n          timeout  = "2s"\n        }\n\n        check {\n          type     = "script"\n          name     = "check_table"\n          command  = "/usr/local/bin/check_mysql_table_status"\n          args     = ["--verbose"]\n          interval = "60s"\n          timeout  = "5s"\n\n          check_restart {\n            limit = 3\n            grace = "90s"\n            ignore_warnings = false\n          }\n        }\n      }\n    }\n  }\n}\n',
    Type: 'hcl',
    Variables: {
      datacenters: ['west'],
      external_port: 4000,
    },
  },
};
