/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import hbs from 'htmlbars-inline-precompile';

export default {
  title: 'Components/Diff Viewer',
};

export let DiffViewerWithInsertions = () => {
  return {
    template: hbs`
      <h5 class="title is-5">Diff Viewer with insertions</h5>
      <div class="boxed-section">
        <div class="boxed-section-body is-dark">
          <JobDiff @diff={{insertionsOnly}} />
        </div>
      </div>
      `,
    context: {
      insertionsOnly: generateDiff([
        {
          Annotations: null,
          Name: 'Attempts',
          New: '15',
          Old: '15',
          Type: 'None',
        },
        {
          Annotations: null,
          Name: 'Delay',
          New: '25000000000',
          Old: '',
          Type: 'Added',
        },
        {
          Annotations: null,
          Name: 'Interval',
          New: '900000000000',
          Old: '',
          Type: 'Added',
        },
        {
          Annotations: null,
          Name: 'Mode',
          New: 'delay',
          Old: 'delay',
          Type: 'None',
        },
      ]),
    },
  };
};

export let DiffViewerWithDeletions = () => {
  return {
    template: hbs`
      <h5 class="title is-5">Diff Viewer with deletions</h5>
      <div class="boxed-section">
        <div class="boxed-section-body is-dark">
          <JobDiff @diff={{deletionsOnly}} />
        </div>
      </div>
      `,
    context: {
      deletionsOnly: generateDiff([
        {
          Annotations: null,
          Name: 'Attempts',
          New: '15',
          Old: '15',
          Type: 'None',
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
          New: '',
          Old: 'delay',
          Type: 'Deleted',
        },
      ]),
    },
  };
};

export let DiffViewerWithEdits = () => {
  return {
    template: hbs`
      <h5 class="title is-5">Diff Viewer with edits</h5>
      <div class="boxed-section">
        <div class="boxed-section-body is-dark">
          <JobDiff @diff={{editsOnly}} />
        </div>
        <p class="annotation">Often times a diff will only have a couple lines. Minor tweaks to a job spec result in small diffs.</p>
      </div>
      `,
    context: {
      editsOnly: generateDiff([
        {
          Annotations: null,
          Name: 'Attempts',
          New: '15',
          Old: '15',
          Type: 'None',
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
          Old: '250000000000',
          Type: 'Edited',
        },
        {
          Annotations: null,
          Name: 'Mode',
          New: 'delay',
          Old: 'delay',
          Type: 'None',
        },
      ]),
    },
  };
};

export let DiffViewerWithManyChanges = () => {
  return {
    template: hbs`
      <h5 class="title is-5">Diff Viewer with many changes</h5>
      <div class="boxed-section">
        <div class="boxed-section-body is-dark">
          <JobDiff @diff={{largeDiff}} />
        </div>
      </div>
      `,
    context: {
      largeDiff: {
        Fields: null,
        ID: 'example',
        Objects: null,
        TaskGroups: [
          {
            Fields: null,
            Name: 'cache',
            Objects: null,
            Tasks: [
              {
                Annotations: null,
                Fields: [
                  {
                    Annotations: null,
                    Name: 'Meta[one]',
                    New: "flew over the cuckoo's nest",
                    Old: '',
                    Type: 'Added',
                  },
                  {
                    Annotations: null,
                    Name: 'Meta[two]',
                    New: 'birds on a wire',
                    Old: '',
                    Type: 'Added',
                  },
                ],
                Name: 'redis',
                Objects: [
                  {
                    Fields: [
                      {
                        Annotations: null,
                        Name: 'image',
                        New: 'redis:3.4',
                        Old: 'redis:7',
                        Type: 'Edited',
                      },
                      {
                        Annotations: null,
                        Name: 'port_map[0][db]',
                        New: '6380',
                        Old: '6379',
                        Type: 'Edited',
                      },
                    ],
                    Name: 'Config',
                    Objects: null,
                    Type: 'Edited',
                  },
                  {
                    Fields: [
                      {
                        Annotations: null,
                        Name: 'CPU',
                        New: '1000',
                        Old: '500',
                        Type: 'Edited',
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
                    Objects: [
                      {
                        Fields: [
                          {
                            Annotations: null,
                            Name: 'MBits',
                            New: '100',
                            Old: '',
                            Type: 'Added',
                          },
                        ],
                        Name: 'Network',
                        Objects: [
                          {
                            Fields: [
                              {
                                Annotations: null,
                                Name: 'Label',
                                New: 'db',
                                Old: '',
                                Type: 'Added',
                              },
                            ],
                            Name: 'Dynamic Port',
                            Objects: null,
                            Type: 'Added',
                          },
                        ],
                        Type: 'Added',
                      },
                      {
                        Fields: [
                          {
                            Annotations: null,
                            Name: 'MBits',
                            New: '',
                            Old: '10',
                            Type: 'Deleted',
                          },
                        ],
                        Name: 'Network',
                        Objects: [
                          {
                            Fields: [
                              {
                                Annotations: null,
                                Name: 'Label',
                                New: '',
                                Old: 'db',
                                Type: 'Deleted',
                              },
                            ],
                            Name: 'Dynamic Port',
                            Objects: null,
                            Type: 'Deleted',
                          },
                        ],
                        Type: 'Deleted',
                      },
                    ],
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
                            Name: 'Tags',
                            New: 'redis',
                            Old: '',
                            Type: 'Added',
                          },
                          {
                            Annotations: null,
                            Name: 'Tags',
                            New: 'cache',
                            Old: 'cache',
                            Type: 'None',
                          },
                          {
                            Annotations: null,
                            Name: 'Tags',
                            New: 'global',
                            Old: 'global',
                            Type: 'None',
                          },
                        ],
                        Name: 'Tags',
                        Objects: null,
                        Type: 'Added',
                      },
                      {
                        Fields: [
                          {
                            Annotations: null,
                            Name: 'AddressMode',
                            New: '',
                            Old: '',
                            Type: 'None',
                          },
                          {
                            Annotations: null,
                            Name: 'Command',
                            New: '',
                            Old: '',
                            Type: 'None',
                          },
                          {
                            Annotations: null,
                            Name: 'GRPCService',
                            New: '',
                            Old: '',
                            Type: 'None',
                          },
                          {
                            Annotations: null,
                            Name: 'GRPCUseTLS',
                            New: 'false',
                            Old: 'false',
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
                            New: '15000000000',
                            Old: '10000000000',
                            Type: 'Edited',
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
                            Name: 'TLSServerName',
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
                            New: '7000000000',
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
          {
            Fields: [
              {
                Annotations: null,
                Name: 'Count',
                New: '1',
                Old: '',
                Type: 'Added',
              },
              {
                Annotations: null,
                Name: 'Meta[key]',
                New: 'value',
                Old: '',
                Type: 'Added',
              },
              {
                Annotations: null,
                Name: 'Meta[red]',
                New: 'fish',
                Old: '',
                Type: 'Added',
              },
            ],
            Name: 'cache2',
            Objects: [
              {
                Fields: [
                  {
                    Annotations: null,
                    Name: 'Attempts',
                    New: '2',
                    Old: '',
                    Type: 'Added',
                  },
                  {
                    Annotations: null,
                    Name: 'Delay',
                    New: '15000000000',
                    Old: '',
                    Type: 'Added',
                  },
                  {
                    Annotations: null,
                    Name: 'Interval',
                    New: '1800000000000',
                    Old: '',
                    Type: 'Added',
                  },
                  {
                    Annotations: null,
                    Name: 'Mode',
                    New: 'fail',
                    Old: '',
                    Type: 'Added',
                  },
                ],
                Name: 'RestartPolicy',
                Objects: null,
                Type: 'Added',
              },
              {
                Fields: [
                  {
                    Annotations: null,
                    Name: 'Migrate',
                    New: 'false',
                    Old: '',
                    Type: 'Added',
                  },
                  {
                    Annotations: null,
                    Name: 'SizeMB',
                    New: '300',
                    Old: '',
                    Type: 'Added',
                  },
                  {
                    Annotations: null,
                    Name: 'Sticky',
                    New: 'false',
                    Old: '',
                    Type: 'Added',
                  },
                ],
                Name: 'EphemeralDisk',
                Objects: null,
                Type: 'Added',
              },
            ],
            Tasks: [
              {
                Annotations: null,
                Fields: [
                  {
                    Annotations: null,
                    Name: 'Driver',
                    New: 'docker',
                    Old: '',
                    Type: 'Added',
                  },
                  {
                    Annotations: null,
                    Name: 'KillTimeout',
                    New: '5000000000',
                    Old: '',
                    Type: 'Added',
                  },
                  {
                    Annotations: null,
                    Name: 'Leader',
                    New: 'false',
                    Old: '',
                    Type: 'Added',
                  },
                  {
                    Annotations: null,
                    Name: 'ShutdownDelay',
                    New: '0',
                    Old: '',
                    Type: 'Added',
                  },
                ],
                Name: 'redis',
                Objects: [
                  {
                    Fields: [
                      {
                        Annotations: null,
                        Name: 'image',
                        New: 'redis:7',
                        Old: '',
                        Type: 'Added',
                      },
                      {
                        Annotations: null,
                        Name: 'port_map[0][db]',
                        New: '6379',
                        Old: '',
                        Type: 'Added',
                      },
                    ],
                    Name: 'Config',
                    Objects: null,
                    Type: 'Added',
                  },
                  {
                    Fields: [
                      {
                        Annotations: null,
                        Name: 'CPU',
                        New: '500',
                        Old: '',
                        Type: 'Added',
                      },
                      {
                        Annotations: null,
                        Name: 'DiskMB',
                        New: '0',
                        Old: '',
                        Type: 'Added',
                      },
                      {
                        Annotations: null,
                        Name: 'IOPS',
                        New: '0',
                        Old: '',
                        Type: 'Added',
                      },
                      {
                        Annotations: null,
                        Name: 'MemoryMB',
                        New: '256',
                        Old: '',
                        Type: 'Added',
                      },
                    ],
                    Name: 'Resources',
                    Objects: [
                      {
                        Fields: [
                          {
                            Annotations: null,
                            Name: 'MBits',
                            New: '10',
                            Old: '',
                            Type: 'Added',
                          },
                        ],
                        Name: 'Network',
                        Objects: [
                          {
                            Fields: [
                              {
                                Annotations: null,
                                Name: 'Label',
                                New: 'db',
                                Old: '',
                                Type: 'Added',
                              },
                            ],
                            Name: 'Dynamic Port',
                            Objects: null,
                            Type: 'Added',
                          },
                        ],
                        Type: 'Added',
                      },
                    ],
                    Type: 'Added',
                  },
                ],
                Type: 'Added',
              },
            ],
            Type: 'Added',
            Updates: null,
          },
        ],
        Type: 'Edited',
      },
    },
  };
};

function generateDiff(changeset) {
  return {
    Fields: null,
    ID: 'insertions-only',
    Objects: null,
    TaskGroups: [
      {
        Fields: [
          {
            Annotations: null,
            Name: 'Count',
            New: '2',
            Old: '2',
            Type: 'None',
          },
        ],
        Name: 'cache',
        Objects: [
          {
            Fields: changeset,
            Name: 'RestartPolicy',
            Objects: null,
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
