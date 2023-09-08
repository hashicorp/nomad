/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

/* eslint-disable ember/avoid-leaking-state-in-ember-objects */
import { module, test } from 'qunit';
import { setupTest } from 'ember-qunit';
import Service from '@ember/service';
import setupAbility from 'nomad-ui/tests/helpers/setup-ability';

module('Unit | Ability | recommendation', function (hooks) {
  setupTest(hooks);
  setupAbility('recommendation')(hooks);

  module(
    'when the Dynamic Application Sizing feature is present',
    function (hooks) {
      hooks.beforeEach(function () {
        const mockSystem = Service.extend({
          features: ['Dynamic Application Sizing'],
        });

        this.owner.register('service:system', mockSystem);
      });

      test('it permits accepting recommendations when ACLs are disabled', function (assert) {
        const mockToken = Service.extend({
          aclEnabled: false,
        });

        this.owner.register('service:token', mockToken);

        assert.ok(this.ability.canAccept);
      });

      test('it permits accepting recommendations for client tokens where any namespace has submit-job capabilities', function (assert) {
        this.owner.lookup('service:system').set('activeNamespace', {
          name: 'anotherNamespace',
        });

        const mockToken = Service.extend({
          aclEnabled: true,
          selfToken: { type: 'client' },
          selfTokenPolicies: [
            {
              rulesJSON: {
                Namespaces: [
                  {
                    Name: 'aNamespace',
                    Capabilities: [],
                  },
                  {
                    Name: 'bNamespace',
                    Capabilities: ['submit-job'],
                  },
                ],
              },
            },
          ],
        });

        this.owner.register('service:token', mockToken);

        assert.ok(this.ability.canAccept);
      });
    }
  );

  module(
    'when the Dynamic Application Sizing feature is not present',
    function (hooks) {
      hooks.beforeEach(function () {
        const mockSystem = Service.extend({
          features: [],
        });

        this.owner.register('service:system', mockSystem);
      });

      test('it does not permit accepting recommendations regardless of ACL status', function (assert) {
        const mockToken = Service.extend({
          aclEnabled: false,
        });

        this.owner.register('service:token', mockToken);

        assert.notOk(this.ability.canAccept);
      });
    }
  );
});
