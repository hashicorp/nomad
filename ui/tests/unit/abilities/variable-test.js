import { module, test } from 'qunit';
import { setupTest } from 'ember-qunit';
import Service from '@ember/service';
import setupAbility from 'nomad-ui/tests/helpers/setup-ability';

module('Unit | Ability | variable', function (hooks) {
  setupTest(hooks);
  setupAbility('variable')(hooks);

  module('when the Variables feature is not present', function (hooks) {
    hooks.beforeEach(function () {
      const mockSystem = Service.extend({
        features: [],
      });

      this.owner.register('service:system', mockSystem);
    });

    test('it does not permit listing variables by default', function (assert) {
      const mockToken = Service.extend({
        aclEnabled: true,
      });

      this.owner.register('service:token', mockToken);

      assert.notOk(this.ability.canList);
    });

    test('it does not permite listing variables when token type is client', function (assert) {
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

    test('it permits listing variables when token has SecureVariables alone is in its rules', function (assert) {
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
});
