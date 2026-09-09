/**
 * Copyright IBM Corp. 2015, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import { module, test } from 'qunit';
import { setupTest } from 'ember-qunit';
import JobModel from 'nomad-ui/models/job';

module('Unit | Serializer | Job', function (hooks) {
  setupTest(hooks);
  hooks.beforeEach(function () {
    this.store = this.owner.lookup('service:store');
    this.subject = () => this.store.serializerFor('job');
  });

  test('group selection statuses retain allocation ownership and desired state', function (assert) {
    const claim = { Name: 'runtime', Slot: 0, Cohort: 'current' };
    const status = {
      runtime: {
        Count: 1,
        Groups: ['encoder', 'thor'],
        Slots: { 0: { TaskGroup: 'thor', Cohort: 'current' } },
        Unplaced: 0,
      },
    };
    const result = this.subject().normalizeQueryResponse(
      this.store,
      JobModel,
      [
        {
          ID: 'example',
          Namespace: 'default',
          GroupCountSum: 1,
          GroupSelectionStatuses: status,
          Allocs: [
            {
              ID: 'allocation',
              Group: 'thor',
              ClientStatus: 'running',
              DesiredStatus: 'run',
              GroupSelection: claim,
              JobVersion: 2,
              DeploymentStatus: {},
            },
          ],
        },
      ],
      null,
      'query',
    );
    assert.deepEqual(result.data[0].attributes.groupSelectionStatuses, status);
    const alloc = this.store.peekRecord('allocation', 'allocation');
    assert.deepEqual(alloc.groupSelection, claim);
    assert.strictEqual(alloc.taskGroupName, 'thor');
    assert.strictEqual(alloc.desiredStatus, 'run');
    assert.strictEqual(alloc.jobVersion, 2);
  });

  test('`default` is used as the namespace in the job ID when there is no namespace in the payload', async function (assert) {
    const original = {
      ID: 'example',
      Name: 'example',
    };

    const { data } = this.subject().normalize(JobModel, original);
    assert.deepEqual(
      data.id,
      JSON.stringify([data.attributes.name, 'default']),
    );
  });

  test('The ID of the record is a composite of both the name and the namespace', async function (assert) {
    const original = {
      ID: 'example',
      Name: 'example',
      Namespace: 'special-namespace',
    };

    const { data } = this.subject().normalize(JobModel, original);
    assert.deepEqual(
      data.id,
      JSON.stringify([
        data.attributes.name,
        data.relationships.namespace.data.id,
      ]),
    );
  });
});
