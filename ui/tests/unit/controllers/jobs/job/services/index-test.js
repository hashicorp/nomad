/**
 * Copyright IBM Corp. 2015, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import { A } from '@ember/array';
import { module, test } from 'qunit';
import { setupTest } from 'ember-qunit';

module('Unit | Controller | jobs/job/services/index', function (hooks) {
  setupTest(hooks);

  test('it builds service wrappers without mutating source fragments', function (assert) {
    const controller = this.owner.lookup('controller:jobs/job/services/index');

    const groupFragment = Object.freeze({
      name: 'group-svc',
      portLabel: 'http',
      tags: ['group'],
      canary_tags: [],
      provider: 'nomad',
      connect: null,
    });

    const taskFragment = Object.freeze({
      name: 'task-svc',
      portLabel: 'http',
      tags: ['task'],
      canary_tags: [],
      provider: 'nomad',
      connect: null,
    });

    controller.set('model', {
      services: A([
        { name: 'group-svc', derivedLevel: 'group' },
        { name: 'task-svc', derivedLevel: 'task' },
      ]),
      taskGroups: A([
        {
          services: A([groupFragment]),
          tasks: A([
            {
              services: A([taskFragment]),
            },
          ]),
        },
      ]),
    });

    const services = controller.services;

    assert.strictEqual(services.length, 2, 'returns both service wrappers');

    const groupService = services.find(
      (service) => service.name === 'group-svc',
    );
    const taskService = services.find((service) => service.name === 'task-svc');

    assert.strictEqual(groupService.level, 'group');
    assert.strictEqual(taskService.level, 'task');
    assert.deepEqual(
      groupService.instances.map((service) => service.derivedLevel),
      ['group'],
    );
    assert.deepEqual(
      taskService.instances.map((service) => service.derivedLevel),
      ['task'],
    );

    assert.notOk(
      Object.prototype.hasOwnProperty.call(groupFragment, 'level'),
      'source group fragment was not mutated',
    );
    assert.notOk(
      Object.prototype.hasOwnProperty.call(groupFragment, 'instances'),
      'source group fragment was not given instances',
    );
    assert.notOk(
      Object.prototype.hasOwnProperty.call(taskFragment, 'level'),
      'source task fragment was not mutated',
    );
    assert.notOk(
      Object.prototype.hasOwnProperty.call(taskFragment, 'instances'),
      'source task fragment was not given instances',
    );
  });
});
