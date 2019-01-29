import { test } from 'ember-qunit';
import EvaluationModel from 'nomad-ui/models/evaluation';
import moduleForSerializer from '../../helpers/module-for-serializer';

moduleForSerializer('evaluation', 'Unit | Serializer | Evaluation', {
  needs: [
    'serializer:evaluation',
    'service:system',
    'transform:fragment-array',
    'model:job',
    'model:placement-failure',
  ],
});

const normalizationTestCases = [
  {
    name: 'Normal',
    in: {
      ID: 'test-eval',
      FailedTGAllocs: {
        taskGroup: {
          NodesAvailable: 10,
        },
      },
      JobID: 'some-job-id',
      Job: {
        Namespace: 'test-namespace',
      },
    },
    out: {
      data: {
        id: 'test-eval',
        type: 'evaluation',
        attributes: {
          failedTGAllocs: [
            {
              name: 'taskGroup',
              nodesAvailable: 10,
            },
          ],
        },
        relationships: {
          job: {
            data: {
              id: '["some-job-id","test-namespace"]',
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
      ID: 'test-eval',
      FailedTGAllocs: {
        'one.two': {
          NodesAvailable: 10,
        },
        'three.four': {
          NodesAvailable: 25,
        },
      },
      JobID: 'some-job-id',
      Job: {
        Namespace: 'test-namespace',
      },
    },
    out: {
      data: {
        id: 'test-eval',
        type: 'evaluation',
        attributes: {
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
        relationships: {
          job: {
            data: {
              id: '["some-job-id","test-namespace"]',
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
    assert.deepEqual(this.subject().normalize(EvaluationModel, testCase.in), testCase.out);
  });
});
