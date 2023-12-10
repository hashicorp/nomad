/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { module, test } from 'qunit';
import { setupTest } from 'ember-qunit';
import ScaleEventModel from 'nomad-ui/models/scale-event';

module('Unit | Serializer | Scale Event', function (hooks) {
  setupTest(hooks);
  hooks.beforeEach(function () {
    this.store = this.owner.lookup('service:store');
    this.subject = () => this.store.serializerFor('scale-event');
  });

  const sampleDate = new Date('2020-12-07T00:00:00');
  const normalizationTestCases = [
    {
      name: 'Normal',
      in: {
        Count: null,
        CreateIndex: 16,
        Error: true,
        EvalID: null,
        Message: 'job scaling blocked due to active deployment',
        Meta: {
          OriginalCount: 3,
          OriginalMessage: 'submitted using the Nomad CLI',
          OriginalMeta: null,
        },
        PreviousCount: 1,
        Time: +sampleDate * 1000000,
      },
      out: {
        data: {
          attributes: {
            count: null,
            error: true,
            message: 'job scaling blocked due to active deployment',
            meta: {
              OriginalCount: 3,
              OriginalMessage: 'submitted using the Nomad CLI',
              OriginalMeta: null,
            },
            previousCount: 1,
            time: sampleDate,
            timeNanos: 0,
          },
          relationships: {},
          type: 'scale-event',
          id: null,
        },
      },
    },
    {
      name: 'No meta',
      in: {
        Count: 3,
        CreateIndex: 23,
        Error: false,
        EvalID: '753bb12c-345e-22b2-f0b4-17f84239b98b',
        Message: 'submitted using the Nomad CLI',
        Meta: null,
        PreviousCount: 1,
        Time: +sampleDate * 1000000,
      },
      out: {
        data: {
          attributes: {
            count: 3,
            error: false,
            message: 'submitted using the Nomad CLI',
            meta: {},
            previousCount: 1,
            time: sampleDate,
            timeNanos: 0,
          },
          relationships: {},
          type: 'scale-event',
          id: null,
        },
      },
    },
  ];

  normalizationTestCases.forEach((testCase) => {
    test(`normalization: ${testCase.name}`, async function (assert) {
      assert.deepEqual(
        this.subject().normalize(ScaleEventModel, testCase.in),
        testCase.out
      );
    });
  });
});
