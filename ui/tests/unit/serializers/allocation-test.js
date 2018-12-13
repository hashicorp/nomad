import { test } from 'ember-qunit';
import AllocationModel from 'nomad-ui/models/allocation';
import moduleForSerializer from '../../helpers/module-for-serializer';

moduleForSerializer('allocation', 'Unit | Serializer | Allocation', {
  needs: [
    'service:token',
    'service:system',
    'serializer:allocation',
    'transform:fragment',
    'transform:fragment-array',
    'model:job',
    'model:node',
    'model:namespace',
    'model:evaluation',
    'model:allocation',
    'model:resources',
    'model:task-state',
    'model:reschedule-event',
  ],
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
  test(`normalization: ${testCase.name}`, function(assert) {
    assert.deepEqual(this.subject().normalize(AllocationModel, testCase.in), testCase.out);
  });
});
