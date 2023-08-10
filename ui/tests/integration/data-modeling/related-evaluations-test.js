/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { setupMirage } from 'ember-cli-mirage/test-support';
import { module, test } from 'qunit';
import { setupTest } from 'ember-qunit';

module('Integration | Data Modeling | related evaluations', function (hooks) {
  setupTest(hooks);
  setupMirage(hooks);

  test('it should a return a list of related evaluations when the related query parameter is specified', async function (assert) {
    assert.expect(2);
    const store = this.owner.lookup('service:store');

    server.get('/evaluation/:id', function (_, fakeRes) {
      assert.equal(
        fakeRes.queryParams.related,
        'true',
        'it should append the related query parameter when making the API request for related evaluations'
      );
      return {
        ID: 'tomster',
        Priority: 50,
        Type: 'service',
        TriggeredBy: 'job-register',
        JobID: 'example',
        JobModifyIndex: 52,
        NodeID: 'yes',
        NodeModifyIndex: 0,
        Status: 'complete',
        StatusDescription: '',
        Wait: 0,
        NextEval: '',
        PreviousEval: '',
        BlockedEval: '',
        FailedTGAllocs: null,
        ClassEligibility: null,
        EscapedComputedClass: false,
        AnnotatePlan: false,
        SnapshotIndex: 53,
        QueuedAllocations: {
          cache: 0,
        },
        CreateIndex: 53,
        ModifyIndex: 55,
        Related: [],
      };
    });
    await store.findRecord('evaluation', 'tomster', {
      adapterOptions: { related: true },
    });

    server.get('/evaluation/:id', function (_, fakeRes) {
      assert.notOk(
        fakeRes.queryParams.related,
        'it should not append the related query parameter when making the API request for related evaluations'
      );
      return {
        ID: 'tomster',
        Priority: 50,
        Type: 'service',
        TriggeredBy: 'job-register',
        JobID: 'example',
        JobModifyIndex: 52,
        NodeID: 'yes',
        NodeModifyIndex: 0,
        Status: 'complete',
        StatusDescription: '',
        Wait: 0,
        NextEval: '',
        PreviousEval: '',
        BlockedEval: '',
        FailedTGAllocs: null,
        ClassEligibility: null,
        EscapedComputedClass: false,
        AnnotatePlan: false,
        SnapshotIndex: 53,
        QueuedAllocations: {
          cache: 0,
        },
        CreateIndex: 53,
        ModifyIndex: 55,
        Related: [],
      };
    });
    await store.findRecord('evaluation', 'tomster');
  });

  test('it should store related evaluations stubs as a hasMany in the store', async function (assert) {
    const store = this.owner.lookup('service:store');

    server.get('/evaluation/:id', function () {
      return {
        ID: 'tomster',
        Priority: 50,
        Type: 'service',
        TriggeredBy: 'job-register',
        JobID: 'example',
        JobModifyIndex: 52,
        NodeID: 'yes',
        NodeModifyIndex: 0,
        Status: 'complete',
        StatusDescription: '',
        Wait: 0,
        NextEval: '',
        PreviousEval: '',
        BlockedEval: '',
        FailedTGAllocs: null,
        ClassEligibility: null,
        EscapedComputedClass: false,
        AnnotatePlan: false,
        SnapshotIndex: 53,
        QueuedAllocations: {
          cache: 0,
        },
        CreateIndex: 53,
        ModifyIndex: 55,
        RelatedEvals: [
          { ID: 'a', StatusDescription: 'a' },
          { ID: 'b', StatusDescription: 'b' },
        ],
      };
    });

    const result = await store.findRecord('evaluation', 'tomster', {
      adapterOptions: { related: true },
    });

    assert.equal(result.relatedEvals.length, 2);

    const mappedResult = result.relatedEvals.map((es) => es.id);

    assert.deepEqual(
      mappedResult,
      ['a', 'b'],
      'related evals data is accessible'
    );
  });
});
