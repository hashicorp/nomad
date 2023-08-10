/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { module, test } from 'qunit';
import { setupTest } from 'ember-qunit';
import EvaluationModel from 'nomad-ui/models/evaluation';

module('Unit | Serializer | Evaluation', function (hooks) {
  setupTest(hooks);
  hooks.beforeEach(function () {
    this.store = this.owner.lookup('service:store');
    this.subject = () => this.store.serializerFor('evaluation');
  });

  const sampleDate = new Date('2018-12-12T00:00:00');
  const normalizationTestCases = [
    {
      name: 'Normal',
      in: {
        ID: 'test-eval',
        CreateTime: +sampleDate * 1000000,
        ModifyTime: +sampleDate * 1000000,
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
            createTime: sampleDate,
            modifyTime: sampleDate,
            failedTGAllocs: [
              {
                name: 'taskGroup',
                nodesAvailable: 10,
              },
            ],
            namespace: 'test-namespace',
            plainJobId: 'some-job-id',
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
        CreateTime: +sampleDate * 1000000,
        ModifyTime: +sampleDate * 1000000,
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
            modifyTime: sampleDate,
            createTime: sampleDate,
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
            namespace: 'test-namespace',
            plainJobId: 'some-job-id',
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

  normalizationTestCases.forEach((testCase) => {
    test(`normalization: ${testCase.name}`, async function (assert) {
      assert.deepEqual(
        this.subject().normalize(EvaluationModel, testCase.in),
        testCase.out
      );
    });
  });
});
