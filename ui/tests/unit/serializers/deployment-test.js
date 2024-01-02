/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { module, test } from 'qunit';
import { setupTest } from 'ember-qunit';
import DeploymentModel from 'nomad-ui/models/deployment';

module('Unit | Serializer | Deployment', function (hooks) {
  setupTest(hooks);
  hooks.beforeEach(function () {
    this.store = this.owner.lookup('service:store');
    this.subject = () => this.store.serializerFor('deployment');
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
                placedCanaryAllocations: [],
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
                placedCanaryAllocations: [],
              },
              {
                name: 'three.four',
                desiredCanaries: 3,
                placedCanaryAllocations: [],
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

  normalizationTestCases.forEach((testCase) => {
    test(`normalization: ${testCase.name}`, async function (assert) {
      assert.deepEqual(
        this.subject().normalize(DeploymentModel, testCase.in),
        testCase.out
      );
    });
  });
});
