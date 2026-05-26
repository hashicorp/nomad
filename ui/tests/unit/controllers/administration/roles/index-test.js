/**
 * Copyright IBM Corp. 2015, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import Service from '@ember/service';
import { module, test } from 'qunit';
import { setupTest } from 'ember-qunit';

module('Unit | Controller | administration/roles/index', function (hooks) {
  setupTest(hooks);

  hooks.beforeEach(function () {
    const notifications = [];

    this.notifications = notifications;

    this.owner.register(
      'service:notifications',
      Service.extend({
        add(notification) {
          notifications.push(notification);
        },
      }),
    );
  });

  test('it returns role wrappers and deletes through the underlying record', async function (assert) {
    const controller = this.owner.lookup(
      'controller:administration/roles/index',
    );

    const roleRecord = {
      id: 'role-1',
      name: 'example-role',
      description: 'Example role',
      policyNames: ['policy-a'],
      deleteRecordCalled: 0,
      saveCalled: 0,
      deleteRecord() {
        this.deleteRecordCalled += 1;
      },
      save() {
        this.saveCalled += 1;
      },
    };

    const token = {
      roles: [roleRecord],
    };

    controller.set('model', {
      roles: [roleRecord],
      tokens: [token],
    });

    const [role] = controller.roles;

    assert.notStrictEqual(role, roleRecord, 'returns a wrapper object');
    assert.strictEqual(role.record, roleRecord, 'keeps the underlying record');
    assert.strictEqual(role.tokens.length, 1, 'preserves the related tokens');
    assert.strictEqual(role.tokens[0], token, 'uses the matching token list');

    await controller.deleteRole.perform(role);

    assert.strictEqual(roleRecord.deleteRecordCalled, 1, 'deletes the record');
    assert.strictEqual(roleRecord.saveCalled, 1, 'saves the record');
    assert.strictEqual(
      this.notifications.length,
      1,
      'emits a success notification',
    );
    assert.strictEqual(
      this.notifications[0].title,
      'Role example-role successfully deleted',
    );
  });
});
