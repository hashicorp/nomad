/* eslint-disable ember/avoid-leaking-state-in-ember-objects */
import { module, test } from 'qunit';
import { setupTest } from 'ember-qunit';
import Service from '@ember/service';
import setupAbility from 'nomad-ui/tests/helpers/setup-ability';

module('Unit | Ability | variable', function (hooks) {
  setupTest(hooks);
  setupAbility('variable')(hooks);
  hooks.beforeEach(function () {
    const mockSystem = Service.extend({
      features: [],
    });

    this.owner.register('service:system', mockSystem);
  });

  module('#list', function () {
    test('it does not permit listing variables by default', function (assert) {
      const mockToken = Service.extend({
        aclEnabled: true,
      });

      this.owner.register('service:token', mockToken);

      assert.notOk(this.ability.canList);
    });

    test('it does not permit listing variables when token type is client', function (assert) {
      const mockToken = Service.extend({
        aclEnabled: true,
        selfToken: { type: 'client' },
      });

      this.owner.register('service:token', mockToken);

      assert.notOk(this.ability.canList);
    });

    test('it permits listing variables when token type is management', function (assert) {
      const mockToken = Service.extend({
        aclEnabled: true,
        selfToken: { type: 'management' },
      });

      this.owner.register('service:token', mockToken);

      assert.ok(this.ability.canList);
    });

    test('it permits listing variables when token has SecureVariables with list capabilities in its rules', function (assert) {
      const mockToken = Service.extend({
        aclEnabled: true,
        selfToken: { type: 'client' },
        selfTokenPolicies: [
          {
            rulesJSON: {
              Namespaces: [
                {
                  Name: 'default',
                  Capabilities: [],
                  SecureVariables: {
                    'Path "*"': {
                      Capabilities: ['list'],
                    },
                  },
                },
              ],
            },
          },
        ],
      });

      this.owner.register('service:token', mockToken);

      assert.ok(this.ability.canList);
    });

    test('it permits listing variables when token has SecureVariables alone in its rules', function (assert) {
      const mockToken = Service.extend({
        aclEnabled: true,
        selfToken: { type: 'client' },
        selfTokenPolicies: [
          {
            rulesJSON: {
              Namespaces: [
                {
                  Name: 'default',
                  Capabilities: [],
                  SecureVariables: {},
                },
              ],
            },
          },
        ],
      });

      this.owner.register('service:token', mockToken);

      assert.ok(this.ability.canList);
    });
  });

  module('#create', function () {
    test('it does not permit creating variables by default', function (assert) {
      const mockToken = Service.extend({
        aclEnabled: true,
      });

      this.owner.register('service:token', mockToken);

      assert.notOk(this.ability.canCreate);
    });

    test('it permits creating variables when token type is management', function (assert) {
      const mockToken = Service.extend({
        aclEnabled: true,
        selfToken: { type: 'management' },
      });

      this.owner.register('service:token', mockToken);

      assert.ok(this.ability.canCreate);
    });

    test('it permits creating variables when acl is disabled', function (assert) {
      const mockToken = Service.extend({
        aclEnabled: false,
        selfToken: { type: 'client' },
      });

      this.owner.register('service:token', mockToken);

      assert.ok(this.ability.canCreate);
    });

    test('it permits creating variables when token has SecureVariables with create capabilities in its rules', function (assert) {
      const mockToken = Service.extend({
        aclEnabled: true,
        selfToken: { type: 'client' },
        selfTokenPolicies: [
          {
            rulesJSON: {
              Namespaces: [
                {
                  Name: 'default',
                  Capabilities: [],
                  SecureVariables: {
                    'Path "*"': {
                      Capabilities: ['create'],
                    },
                  },
                },
              ],
            },
          },
        ],
      });

      this.owner.register('service:token', mockToken);

      assert.ok(this.ability.canCreate);
    });
  });
});
