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

  module('#_nearestMatchingPath', function () {
    test('returns capabilities for an exact path match', function (assert) {
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
                    'Path "foo"': {
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
      const path = 'foo';

      const nearestMatchingPath = this.ability._nearestMatchingPath(path);

      assert.equal(
        nearestMatchingPath,
        'foo',
        'It should return the exact path match.'
      );
    });

    test('returns capabilities for the nearest ancestor if no exact match', function (assert) {
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
                    'Path "foo/*"': {
                      Capabilities: ['create'],
                    },
                    'Path "foo/bar/*"': {
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
      const path = 'foo/bar/baz';

      const nearestMatchingPath = this.ability._nearestMatchingPath(path);

      assert.equal(
        nearestMatchingPath,
        'foo/bar/*',
        'It should return the nearest ancestor matching path.'
      );
    });

    test('handles wildcard prefix matches', function (assert) {
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
                    'Path "foo/*"': {
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
      const path = 'foo/bar/baz';

      const nearestMatchingPath = this.ability._nearestMatchingPath(path);

      assert.equal(
        nearestMatchingPath,
        'foo/*',
        'It should handle wildcard glob prefixes.'
      );
    });

    test('handles wildcard suffix matches', function (assert) {
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
                    'Path "*/bar"': {
                      Capabilities: ['create'],
                    },
                    'Path "*/bar/baz"': {
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
      const path = 'foo/bar/baz';

      const nearestMatchingPath = this.ability._nearestMatchingPath(path);

      assert.equal(
        nearestMatchingPath,
        '*/bar/baz',
        'It should return the nearest ancestor matching path.'
      );
    });

    test('prioritizes wildcard suffix matches over wildcard prefix matches', function (assert) {
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
                    'Path "*/bar"': {
                      Capabilities: ['create'],
                    },
                    'Path "foo/*"': {
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
      const path = 'foo/bar/baz';

      const nearestMatchingPath = this.ability._nearestMatchingPath(path);

      assert.equal(
        nearestMatchingPath,
        'foo/*',
        'It should prioritize suffix glob wildcard of prefix glob wildcard.'
      );
    });

    test('defaults to the glob path if there is no exact match or wildcard matches', function (assert) {
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
                    'Path "foo"': {
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
      const path = 'foo/bar/baz';

      const nearestMatchingPath = this.ability._nearestMatchingPath(path);

      assert.equal(
        nearestMatchingPath,
        '*',
        'It should default to glob wildcard if no matches.'
      );
    });
  });
});
