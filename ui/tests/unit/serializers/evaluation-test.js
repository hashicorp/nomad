import { module, test } from 'qunit';
import { setupTest } from 'ember-qunit';
import EvaluationModel from 'nomad-ui/models/evaluation';

module('Unit | Serializer | Evaluation', function(hooks) {
  setupTest(hooks);
  hooks.beforeEach(function() {
    this.store = this.owner.lookup('service:store');
    this.subject = () => this.store.serializerFor('evaluation');
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
    test(`normalization: ${testCase.name}`, async function(assert) {
      assert.deepEqual(this.subject().normalize(EvaluationModel, testCase.in), testCase.out);
    });
  });
});
