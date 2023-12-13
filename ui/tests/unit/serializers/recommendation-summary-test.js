/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { module, test } from 'qunit';
import { setupTest } from 'ember-qunit';
import RecommendationSummaryModel from 'nomad-ui/models/recommendation-summary';

module('Unit | Serializer | RecommendationSummary', function (hooks) {
  setupTest(hooks);
  hooks.beforeEach(function () {
    this.store = this.owner.lookup('service:store');
    this.subject = () => this.store.serializerFor('recommendation-summary');
  });

  const normalizationTestCases = [
    {
      name: 'Normal',
      in: [
        {
          ID: '2345',
          JobID: 'job-id',
          Namespace: 'default',
          Region: 'us-east-1',
          Group: 'group-1',
          Task: 'task-1',
          Resource: 'MemoryMB',
          Value: 500,
          Current: 1000,
          Stats: {
            min: 25.0,
            max: 575.0,
            mean: 425.0,
            media: 40.0,
          },
          SubmitTime: 1600000002000000000,
        },
        {
          ID: '1234',
          JobID: 'job-id',
          Namespace: 'default',
          Region: 'us-east-1',
          Group: 'group-1',
          Task: 'task-1',
          Resource: 'CPU',
          Value: 500,
          Current: 1000,
          Stats: {
            min: 25.0,
            max: 575.0,
            mean: 425.0,
            media: 40.0,
          },
          SubmitTime: 1600000001000000000,
        },
        {
          ID: '6789',
          JobID: 'other-job-id',
          Namespace: 'other',
          Region: 'us-east-1',
          Group: 'group-2',
          Task: 'task-2',
          Resource: 'MemoryMB',
          Value: 500,
          Current: 1000,
          Stats: {
            min: 25.0,
            max: 575.0,
            mean: 425.0,
            media: 40.0,
          },
          SubmitTime: 1600000003000000000,
        },
      ],
      out: {
        data: [
          {
            attributes: {
              jobId: 'job-id',
              jobNamespace: 'default',
              submitTime: new Date(1600000002000),
              taskGroupName: 'group-1',
            },
            id: '1234-2345',
            relationships: {
              job: {
                data: {
                  id: '["job-id","default"]',
                  type: 'job',
                },
              },
              recommendations: {
                data: [
                  {
                    id: '2345',
                    type: 'recommendation',
                  },
                  {
                    id: '1234',
                    type: 'recommendation',
                  },
                ],
              },
            },
            type: 'recommendation-summary',
          },
          {
            attributes: {
              jobId: 'other-job-id',
              jobNamespace: 'other',
              submitTime: new Date(1600000003000),
              taskGroupName: 'group-2',
            },
            id: '6789',
            relationships: {
              job: {
                data: {
                  id: '["other-job-id","other"]',
                  type: 'job',
                },
              },
              recommendations: {
                data: [
                  {
                    id: '6789',
                    type: 'recommendation',
                  },
                ],
              },
            },
            type: 'recommendation-summary',
          },
        ],
        included: [
          {
            attributes: {
              resource: 'MemoryMB',
              stats: {
                max: 575,
                mean: 425,
                media: 40,
                min: 25,
              },
              submitTime: new Date(1600000002000),
              taskName: 'task-1',
              value: 500,
            },
            id: '2345',
            relationships: {
              job: {
                links: {
                  related: '/v1/job/job-id',
                },
              },
            },
            type: 'recommendation',
          },
          {
            attributes: {
              resource: 'CPU',
              stats: {
                max: 575,
                mean: 425,
                media: 40,
                min: 25,
              },
              submitTime: new Date(1600000001000),
              taskName: 'task-1',
              value: 500,
            },
            id: '1234',
            relationships: {
              job: {
                links: {
                  related: '/v1/job/job-id',
                },
              },
            },
            type: 'recommendation',
          },
          {
            attributes: {
              resource: 'MemoryMB',
              stats: {
                max: 575,
                mean: 425,
                media: 40,
                min: 25,
              },
              submitTime: new Date(1600000003000),
              taskName: 'task-2',
              value: 500,
            },
            id: '6789',
            relationships: {
              job: {
                links: {
                  related: '/v1/job/other-job-id?namespace=other',
                },
              },
            },
            type: 'recommendation',
          },
        ],
      },
    },
  ];

  normalizationTestCases.forEach((testCase) => {
    test(`normalization: ${testCase.name}`, async function (assert) {
      assert.deepEqual(
        this.subject().normalizeArrayResponse(
          this.store,
          RecommendationSummaryModel,
          testCase.in
        ),
        testCase.out
      );
    });
  });
});
