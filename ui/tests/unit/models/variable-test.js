/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { module, test } from 'qunit';
import { setupTest } from 'ember-qunit';

module('Unit | Model | variable', function (hooks) {
  setupTest(hooks);

  test('it has basic fetchable properties', function (assert) {
    let store = this.owner.lookup('service:store');

    let model = store.createRecord('variable');
    model.setProperties({
      path: 'my/fun/path',
      namespace: 'default',
      keyValues: [
        { key: 'foo', value: 'bar' },
        { key: 'myVar', value: 'myValue' },
      ],
    });
    assert.ok(model.path);
    assert.equal(model.keyValues.length, 2);
  });

  test('it has a single keyValue by default', function (assert) {
    let store = this.owner.lookup('service:store');

    let model = store.createRecord('variable');
    model.setProperties({
      path: 'my/fun/path',
      namespace: 'default',
    });
    assert.equal(model.keyValues.length, 1);
  });

  test('it correctly moves between keyValues and items', function (assert) {
    let store = this.owner.lookup('service:store');

    let model = store.createRecord('variable');
    model.setProperties({
      path: 'my/fun/path',
      keyValues: [
        { key: 'foo', value: 'bar' },
        { key: 'myVar', value: 'myValue' },
      ],
    });
    assert.equal(model.keyValues.length, 2);
    assert.equal(Object.entries(model.items)[0][0], 'foo');
    assert.equal(Object.entries(model.items)[0][1], 'bar');
    assert.equal(Object.entries(model.items)[1][0], 'myVar');
    assert.equal(Object.entries(model.items)[1][1], 'myValue');
  });

  test('it computes linked entities', function (assert) {
    let store = this.owner.lookup('service:store');

    let model = store.createRecord('variable');
    model.setProperties({
      path: 'nomad/jobs/my-job-name/my-group-name/my-task-name',
    });
    assert.ok(model.pathLinkedEntities, 'generates a linked entities object');
    assert.equal(
      model.pathLinkedEntities.job,
      'my-job-name',
      'identifies the job name'
    );
    assert.equal(
      model.pathLinkedEntities.group,
      'my-group-name',
      'identifies the group name'
    );
    assert.equal(
      model.pathLinkedEntities.task,
      'my-task-name',
      'identifies the task name'
    );

    model.setProperties({
      path: 'nomad/jobs/my-job-name/my-group-name/my-task-name/too-long/oh-no',
    });
    assert.equal(
      model.pathLinkedEntities.job,
      '',
      'entities object lacks a job name if path goes beyond task'
    );
    assert.equal(
      model.pathLinkedEntities.group,
      '',
      'entities object lacks a group name if path goes beyond task'
    );
    assert.equal(
      model.pathLinkedEntities.task,
      '',
      'entities object lacks a task name if path goes beyond task'
    );

    model.setProperties({
      path: 'projects/some/job',
    });
    assert.ok(model.pathLinkedEntities, 'generates a linked entities object');
    assert.equal(
      model.pathLinkedEntities.job,
      '',
      'entities object lacks a job name if not prefixed with nomad/jobs/'
    );
    assert.equal(
      model.pathLinkedEntities.group,
      '',
      'entities object lacks a group name if not prefixed with nomad/jobs/'
    );
    assert.equal(
      model.pathLinkedEntities.task,
      '',
      'entities object lacks a task name if not prefixed with nomad/jobs/'
    );
  });
});
