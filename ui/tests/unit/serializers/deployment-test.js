import { test } from 'ember-qunit';
import DeploymentModel from 'nomad-ui/models/deployment';
import moduleForSerializer from '../../helpers/module-for-serializer';

moduleForSerializer('deployment', 'Unit | Serializer | Deployment', {
  needs: [
    'adapter:application',
    'serializer:deployment',
    'service:system',
    'service:token',
    'transform:fragment-array',
    'model:allocation',
    'model:job',
    'model:task-group-deployment-summary',
  ],
});

const normalizationTestCases = [
  {
    name: 'Normal',
    in: {
      ID: 'test-deployment',
      JobID: 'test-job',
      Namespace: 'test-namespace',
      Status: 'canceled',
      TaskGroups: {
        taskGroup: {
          DesiredCanaries: 2,
        },
      },
    },
    out: {
      data: {
        id: 'test-deployment',
        type: 'deployment',
        attributes: {
          status: 'canceled',
          taskGroupSummaries: [
            {
              name: 'taskGroup',
              desiredCanaries: 2,
            },
          ],
        },
        relationships: {
          allocations: {
            links: {
              related: '/v1/deployment/allocations/test-deployment',
            },
          },
          job: {
            data: {
              id: '["test-job","test-namespace"]',
              type: 'job',
            },
          },
          jobForLatest: {
            data: {
              id: '["test-job","test-namespace"]',
              type: 'job',
            },
          },
        },
      },
    },
  },

  {
    name: 'Dots in task group names',
    in: {
      ID: 'test-deployment',
      JobID: 'test-job',
      Namespace: 'test-namespace',
      Status: 'canceled',
      TaskGroups: {
        'one.two': {
          DesiredCanaries: 2,
        },
        'three.four': {
          DesiredCanaries: 3,
        },
      },
    },
    out: {
      data: {
        id: 'test-deployment',
        type: 'deployment',
        attributes: {
          status: 'canceled',
          taskGroupSummaries: [
            {
              name: 'one.two',
              desiredCanaries: 2,
            },
            {
              name: 'three.four',
              desiredCanaries: 3,
            },
          ],
        },
        relationships: {
          allocations: {
            links: {
              related: '/v1/deployment/allocations/test-deployment',
            },
          },
          job: {
            data: {
              id: '["test-job","test-namespace"]',
              type: 'job',
            },
          },
          jobForLatest: {
            data: {
              id: '["test-job","test-namespace"]',
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
    assert.deepEqual(this.subject().normalize(DeploymentModel, testCase.in), testCase.out);
  });
});
