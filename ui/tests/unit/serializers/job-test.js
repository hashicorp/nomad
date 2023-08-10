/**
 * Copyright (c) HashiCorp, Inc.
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

  test('`default` is used as the namespace in the job ID when there is no namespace in the payload', async function (assert) {
    const original = {
      ID: 'example',
      Name: 'example',
    };

    const { data } = this.subject().normalize(JobModel, original);
    assert.equal(data.id, JSON.stringify([data.attributes.name, 'default']));
  });

  test('The ID of the record is a composite of both the name and the namespace', async function (assert) {
    const original = {
      ID: 'example',
      Name: 'example',
      Namespace: 'special-namespace',
    };

    const { data } = this.subject().normalize(JobModel, original);
    assert.equal(
      data.id,
      JSON.stringify([
        data.attributes.name,
        data.relationships.namespace.data.id,
      ])
    );
  });
});
