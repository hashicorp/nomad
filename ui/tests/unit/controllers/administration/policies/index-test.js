/**
 * Copyright IBM Corp. 2015, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import Service from '@ember/service';
import { module, test } from 'qunit';
import { setupTest } from 'ember-qunit';

module('Unit | Controller | administration/policies/index', function (hooks) {
  setupTest(hooks);

  hooks.beforeEach(function () {
    const notifications = [];
    const removedFromRoles = [];
    const removedFromTokens = [];

    this.notifications = notifications;
    this.removedFromRoles = removedFromRoles;
    this.removedFromTokens = removedFromTokens;

    this.owner.register(
      'service:notifications',
      Service.extend({
        add(notification) {
          notifications.push(notification);
        },
      }),
    );

    this.owner.register(
      'service:store',
      Service.extend({
        peekAll(type) {
          if (type === 'role') {
            return [
              {
                policies: {
                  removeObject(policy) {
                    removedFromRoles.push(policy);
                  },
                },
              },
            ];
          }

          if (type === 'token') {
            return [
              {
                policies: {
                  removeObject(policy) {
                    removedFromTokens.push(policy);
                  },
                },
              },
            ];
          }

          return [];
        },

        peekRecord(type, id) {
          return type === 'policy' && id === 'policy-1'
            ? this.policyRecord
            : null;
        },

        unloadRecord(record) {
          this.unloadedRecord = record;
        },
      }),
    );
  });

  test('it returns policy wrappers and deletes through the underlying record', async function (assert) {
    const controller = this.owner.lookup(
      'controller:administration/policies/index',
    );
    const store = this.owner.lookup('service:store');

    const policyRecord = {
      id: 'policy-1',
      name: 'example-policy',
      description: 'Example policy',
      rules: 'allow = true',
      rulesJSON: { allow: true },
      deleteRecordCalled: 0,
      saveCalled: 0,
      deleteRecord() {
        this.deleteRecordCalled += 1;
      },
      save() {
        this.saveCalled += 1;
      },
    };

    store.policyRecord = policyRecord;

    const token = {
      policies: [policyRecord],
    };

    controller.set('model', {
      policies: [policyRecord],
      tokens: [token],
    });

    const [policy] = controller.policies;

    assert.notStrictEqual(policy, policyRecord, 'returns a wrapper object');
    assert.strictEqual(
      policy.record,
      policyRecord,
      'keeps the underlying record',
    );
    assert.strictEqual(policy.tokens.length, 1, 'preserves the related tokens');
    assert.strictEqual(policy.tokens[0], token, 'uses the matching token list');

    await controller.deletePolicy.perform(policy);

    assert.strictEqual(
      policyRecord.deleteRecordCalled,
      1,
      'deletes the record',
    );
    assert.strictEqual(policyRecord.saveCalled, 1, 'saves the record');
    assert.deepEqual(
      this.removedFromRoles,
      [policyRecord],
      'removes the policy from role associations',
    );
    assert.deepEqual(
      this.removedFromTokens,
      [policyRecord],
      'removes the policy from token associations',
    );
    assert.strictEqual(
      store.unloadedRecord,
      policyRecord,
      'unloads the record',
    );
    assert.strictEqual(
      this.notifications.length,
      1,
      'emits a success notification',
    );
    assert.strictEqual(
      this.notifications[0].title,
      'Policy example-policy successfully deleted',
    );
  });
});
