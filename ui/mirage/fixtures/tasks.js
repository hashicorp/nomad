export default [
  {
    Resources: {
      CPU: 500,
      MemoryMB: 8192,
      DiskMB: 0,
      IOPS: 0,
      Networks: [
        {
          Device: 'eth5',
          CIDR: '',
          IP: '16.89.167.91',
          MBits: 10,
          ReservedPorts: [{ Label: 'monitor', Value: 34300 }, { Label: 'array', Value: 21657 }],
          DynamicPorts: [{ Label: 'firewall', Value: 15134 }, { Label: 'circuit', Value: 53434 }],
        },
        {
          Device: 'eth1',
          CIDR: '',
          IP: '244.225.151.117',
          MBits: 10,
          ReservedPorts: [{ Label: 'matrix', Value: 11962 }],
          DynamicPorts: [{ Label: 'monitor', Value: 26677 }],
        },
        {
          Device: 'eth0',
          CIDR: '',
          IP: '103.251.70.230',
          MBits: 10,
          ReservedPorts: [{ Label: 'firewall', Value: 52969 }, { Label: 'alarm', Value: 18089 }],
          DynamicPorts: [
            { Label: 'firewall', Value: 48544 },
            { Label: 'application', Value: 20472 },
          ],
        },
      ],
    },
    name: 'fixture-task-monitor-0',
    JobID: '',
    groupNames: [],
    taskGroupId: '1000',
    id: '1000',
  },
];
