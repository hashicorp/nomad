import { module, test } from 'qunit';
import { setupTest } from 'ember-qunit';
import AllocationModel from 'nomad-ui/models/allocation';

module('Unit | Serializer | Allocation', function(hooks) {
  setupTest(hooks);
  hooks.beforeEach(function() {
    this.store = this.owner.lookup('service:store');
    this.subject = () => this.store.serializerFor('allocation');
  });

  const sampleDate = new Date('2018-12-12T00:00:00');
  const normalizationTestCases = [
    {
      name: 'Normal',
      in: {
        ID: 'test-allocation',
        JobID: 'test-summary',
        Name: 'test-summary[1]',
        Namespace: 'test-namespace',
        TaskGroup: 'test-group',
        CreateTime: +sampleDate * 1000000,
        ModifyTime: +sampleDate * 1000000,
        TaskStates: {
          testTask: {
            State: 'running',
            Failed: false,
          },
        },
      },
      out: {
        data: {
          id: 'test-allocation',
          type: 'allocation',
          attributes: {
            taskGroupName: 'test-group',
            name: 'test-summary[1]',
            modifyTime: sampleDate,
            createTime: sampleDate,
            states: [
              {
                name: 'testTask',
                state: 'running',
                failed: false,
              },
            ],
            wasPreempted: false,
          },
          relationships: {
            followUpEvaluation: {
              data: null,
            },
            nextAllocation: {
              data: null,
            },
            previousAllocation: {
              data: null,
            },
            preemptedAllocations: {
              data: [],
            },
            preemptedByAllocation: {
              data: null,
            },
            job: {
              data: {
                id: '["test-summary","test-namespace"]',
                type: 'job',
              },
            },
          },
        },
      },
    },

    {
      name: 'Dots in task names',
      in: {
        ID: 'test-allocation',
        JobID: 'test-summary',
        Name: 'test-summary[1]',
        Namespace: 'test-namespace',
        TaskGroup: 'test-group',
        CreateTime: +sampleDate * 1000000,
        ModifyTime: +sampleDate * 1000000,
        TaskStates: {
          'one.two': {
            State: 'running',
            Failed: false,
          },
          'three.four': {
            State: 'pending',
            Failed: true,
          },
        },
      },
      out: {
        data: {
          id: 'test-allocation',
          type: 'allocation',
          attributes: {
            taskGroupName: 'test-group',
            name: 'test-summary[1]',
            modifyTime: sampleDate,
            createTime: sampleDate,
            states: [
              {
                name: 'one.two',
                state: 'running',
                failed: false,
              },
              {
                name: 'three.four',
                state: 'pending',
                failed: true,
              },
            ],
            wasPreempted: false,
          },
          relationships: {
            followUpEvaluation: {
              data: null,
            },
            nextAllocation: {
              data: null,
            },
            previousAllocation: {
              data: null,
            },
            preemptedAllocations: {
              data: [],
            },
            preemptedByAllocation: {
              data: null,
            },
            job: {
              data: {
                id: '["test-summary","test-namespace"]',
                type: 'job',
              },
            },
          },
        },
      },
    },

    {
      name: 'With preemptions',
      in: {
        ID: 'test-allocation',
        JobID: 'test-summary',
        Name: 'test-summary[1]',
        Namespace: 'test-namespace',
        TaskGroup: 'test-group',
        CreateTime: +sampleDate * 1000000,
        ModifyTime: +sampleDate * 1000000,
        TaskStates: {
          task: {
            State: 'running',
            Failed: false,
          },
        },
        PreemptedByAllocation: 'preempter-allocation',
        PreemptedAllocations: ['preempted-one-allocation', 'preempted-two-allocation'],
      },
      out: {
        data: {
          id: 'test-allocation',
          type: 'allocation',
          attributes: {
            taskGroupName: 'test-group',
            name: 'test-summary[1]',
            modifyTime: sampleDate,
            createTime: sampleDate,
            states: [
              {
                name: 'task',
                state: 'running',
                failed: false,
              },
            ],
            wasPreempted: true,
          },
          relationships: {
            followUpEvaluation: {
              data: null,
            },
            nextAllocation: {
              data: null,
            },
            previousAllocation: {
              data: null,
            },
            preemptedAllocations: {
              data: [
                { id: 'preempted-one-allocation', type: 'allocation' },
                { id: 'preempted-two-allocation', type: 'allocation' },
              ],
            },
            preemptedByAllocation: {
              data: {
                id: 'preempter-allocation',
                type: 'allocation',
              },
            },
            job: {
              data: {
                id: '["test-summary","test-namespace"]',
                type: 'job',
              },
            },
          },
        },
      },
    },
  ];

  normalizationTestCases.forEach(testCase => {
    test(`normalization: ${testCase.name}`, async function(assert) {
      assert.deepEqual(this.subject().normalize(AllocationModel, testCase.in), testCase.out);
    });
  });
});
