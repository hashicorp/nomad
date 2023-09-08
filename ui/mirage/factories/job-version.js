/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { Factory } from 'ember-cli-mirage';

import faker from 'nomad-ui/mirage/faker';

const REF_TIME = new Date();

export default Factory.extend({
  stable: faker.random.boolean,
  submitTime: () => faker.date.past(2 / 365, REF_TIME) * 1000000,
  diff() {
    return generateDiff(this.jobId);
  },

  jobId: null,
  version: 0,

  // ID is used for record tracking within Mirage,
  // but Nomad uses the JobID as the version ID.
  tempVersionId() {
    return this.job.id;
  },

  // Directive to restrict any related deployments from having a 'running' status
  noActiveDeployment: false,

  // Directive to restrict any related deployments from having a status other than 'running'
  activeDeployment: false,

  afterCreate(version, server) {
    const args = [
      'deployment',
      version.noActiveDeployment && 'notActive',
      version.activeDeployment && 'active',
      {
        jobId: version.jobId,
        namespace: version.job.namespace,
        versionNumber: version.version,
      },
    ].compact();
    server.create(...args);
  },
});

export function generateDiff(id) {
  return {
    Fields: null,
    ID: id,
    Objects: null,
    TaskGroups: [
      {
        Fields: [
          {
            Annotations: null,
            Name: 'Count',
            New: '2',
            Old: '4',
            Type: 'Edited',
          },
        ],
        Name: 'cache',
        Objects: [
          {
            Fields: [
              {
                Annotations: null,
                Name: 'Attempts',
                New: '15',
                Old: '10',
                Type: 'Edited',
              },
              {
                Annotations: null,
                Name: 'Delay',
                New: '25000000000',
                Old: '25000000000',
                Type: 'None',
              },
              {
                Annotations: null,
                Name: 'Interval',
                New: '900000000000',
                Old: '900000000000',
                Type: 'None',
              },
              {
                Annotations: null,
                Name: 'Mode',
                New: 'delay',
                Old: 'delay',
                Type: 'None',
              },
            ],
            Name: 'RestartPolicy',
            Objects: null,
            Type: 'Edited',
          },
        ],
        Tasks: [
          {
            Annotations: null,
            Fields: null,
            Name: 'redis',
            Objects: [
              {
                Fields: [
                  {
                    Annotations: null,
                    Name: 'CPU',
                    New: '500',
                    Old: '500',
                    Type: 'None',
                  },
                  {
                    Annotations: null,
                    Name: 'DiskMB',
                    New: '0',
                    Old: '0',
                    Type: 'None',
                  },
                  {
                    Annotations: null,
                    Name: 'IOPS',
                    New: '0',
                    Old: '0',
                    Type: 'None',
                  },
                  {
                    Annotations: null,
                    Name: 'MemoryMB',
                    New: '512',
                    Old: '256',
                    Type: 'Edited',
                  },
                ],
                Name: 'Resources',
                Objects: null,
                Type: 'Edited',
              },
              {
                Fields: [
                  {
                    Annotations: null,
                    Name: 'MaxFileSizeMB',
                    New: '15',
                    Old: '10',
                    Type: 'Edited',
                  },
                  {
                    Annotations: null,
                    Name: 'MaxFiles',
                    New: '10',
                    Old: '10',
                    Type: 'None',
                  },
                ],
                Name: 'LogConfig',
                Objects: null,
                Type: 'Edited',
              },
              {
                Fields: [
                  {
                    Annotations: null,
                    Name: 'AddressMode',
                    New: 'auto',
                    Old: 'auto',
                    Type: 'None',
                  },
                  {
                    Annotations: null,
                    Name: 'Name',
                    New: 'redis-cache',
                    Old: 'redis-cache',
                    Type: 'None',
                  },
                  {
                    Annotations: null,
                    Name: 'PortLabel',
                    New: 'db',
                    Old: 'db',
                    Type: 'None',
                  },
                ],
                Name: 'Service',
                Objects: [
                  {
                    Fields: [
                      {
                        Annotations: null,
                        Name: 'Command',
                        New: '',
                        Old: '',
                        Type: 'None',
                      },
                      {
                        Annotations: null,
                        Name: 'InitialStatus',
                        New: '',
                        Old: '',
                        Type: 'None',
                      },
                      {
                        Annotations: null,
                        Name: 'Interval',
                        New: '10000000000',
                        Old: '10000000000',
                        Type: 'None',
                      },
                      {
                        Annotations: null,
                        Name: 'Method',
                        New: '',
                        Old: '',
                        Type: 'None',
                      },
                      {
                        Annotations: null,
                        Name: 'Name',
                        New: 'alive',
                        Old: 'alive',
                        Type: 'None',
                      },
                      {
                        Annotations: null,
                        Name: 'Path',
                        New: '',
                        Old: '',
                        Type: 'None',
                      },
                      {
                        Annotations: null,
                        Name: 'PortLabel',
                        New: '',
                        Old: '',
                        Type: 'None',
                      },
                      {
                        Annotations: null,
                        Name: 'Protocol',
                        New: '',
                        Old: '',
                        Type: 'None',
                      },
                      {
                        Annotations: null,
                        Name: 'TLSSkipVerify',
                        New: 'false',
                        Old: 'false',
                        Type: 'None',
                      },
                      {
                        Annotations: null,
                        Name: 'Timeout',
                        New: '3000000000',
                        Old: '2000000000',
                        Type: 'Edited',
                      },
                      {
                        Annotations: null,
                        Name: 'Type',
                        New: 'tcp',
                        Old: 'tcp',
                        Type: 'None',
                      },
                    ],
                    Name: 'Check',
                    Objects: null,
                    Type: 'Edited',
                  },
                ],
                Type: 'Edited',
              },
            ],
            Type: 'Edited',
          },
        ],
        Type: 'Edited',
        Updates: null,
      },
    ],
    Type: 'Edited',
  };
}
