import { test } from 'ember-qunit';
import JobPlanModel from 'nomad-ui/models/job-plan';
import moduleForSerializer from '../../helpers/module-for-serializer';

moduleForSerializer('job-plan', 'Unit | Serializer | JobPlan', {
  needs: [
    'service:token',
    'service:system',
    'serializer:job-plan',
    'transform:fragment-array',
    'model:placement-failure',
  ],
});

const normalizationTestCases = [
  {
    name: 'Normal',
    in: {
      ID: 'test-plan',
      Diff: {
        Arbitrary: 'Value',
      },
      FailedTGAllocs: {
        taskGroup: {
          NodesAvailable: 10,
        },
      },
    },
    out: {
      data: {
        id: 'test-plan',
        type: 'job-plan',
        attributes: {
          diff: {
            Arbitrary: 'Value',
          },
          failedTGAllocs: [
            {
              name: 'taskGroup',
              nodesAvailable: 10,
            },
          ],
        },
        relationships: {},
      },
    },
  },

  {
    name: 'Dots in task names',
    in: {
      ID: 'test-plan',
      Diff: {
        Arbitrary: 'Value',
      },
      FailedTGAllocs: {
        'one.two': {
          NodesAvailable: 10,
        },
        'three.four': {
          NodesAvailable: 25,
        },
      },
    },
    out: {
      data: {
        id: 'test-plan',
        type: 'job-plan',
        attributes: {
          diff: {
            Arbitrary: 'Value',
          },
          failedTGAllocs: [
            {
              name: 'one.two',
              nodesAvailable: 10,
            },
            {
              name: 'three.four',
              nodesAvailable: 25,
            },
          ],
        },
        relationships: {},
      },
    },
  },
];

normalizationTestCases.forEach(testCase => {
  test(`normalization: ${testCase.name}`, function(assert) {
    assert.deepEqual(this.subject().normalize(JobPlanModel, testCase.in), testCase.out);
  });
});
