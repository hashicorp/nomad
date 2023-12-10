/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { module, test } from 'qunit';
import { setupTest } from 'ember-qunit';
import JobSummaryModel from 'nomad-ui/models/job-summary';

module('Unit | Serializer | JobSummary', function (hooks) {
  setupTest(hooks);
  hooks.beforeEach(function () {
    this.store = this.owner.lookup('service:store');
    this.subject = () => this.store.serializerFor('job-summary');
  });

  const normalizationTestCases = [
    {
      name: 'Normal',
      in: {
        JobID: 'test-summary',
        Namespace: 'test-namespace',
        Summary: {
          taskGroup: {
            Complete: 0,
            Running: 1,
          },
        },
      },
      out: {
        data: {
          id: '["test-summary","test-namespace"]',
          type: 'job-summary',
          attributes: {
            taskGroupSummaries: [
              {
                name: 'taskGroup',
                completeAllocs: 0,
                runningAllocs: 1,
              },
            ],
          },
          relationships: {
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
      name: 'Dots in task group names',
      in: {
        JobID: 'test-summary',
        Namespace: 'test-namespace',
        Summary: {
          'one.two': {
            Complete: 0,
            Running: 1,
          },
          'three.four': {
            Failed: 2,
            Lost: 3,
          },
        },
      },
      out: {
        data: {
          id: '["test-summary","test-namespace"]',
          type: 'job-summary',
          attributes: {
            taskGroupSummaries: [
              {
                name: 'one.two',
                completeAllocs: 0,
                runningAllocs: 1,
              },
              {
                name: 'three.four',
                failedAllocs: 2,
                lostAllocs: 3,
              },
            ],
          },
          relationships: {
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

  normalizationTestCases.forEach((testCase) => {
    test(`normalization: ${testCase.name}`, async function (assert) {
      assert.deepEqual(
        this.subject().normalize(JobSummaryModel, testCase.in),
        testCase.out
      );
    });
  });
});
