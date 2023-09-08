/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

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

    test('it permits listing variables when token has Variables with list capabilities in its rules', function (assert) {
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
                  Variables: {
                    Paths: [{ Capabilities: ['list'], PathSpec: '*' }],
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

    test('it does not permit listing variables when token has Variables alone in its rules', function (assert) {
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
                  Variables: {},
                },
              ],
            },
          },
        ],
      });

      this.owner.register('service:token', mockToken);

      assert.notOk(this.ability.canList);
    });

    test('it does not permit listing variables when token has a null Variables block', function (assert) {
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
                  Variables: null,
                },
              ],
            },
          },
        ],
      });

      this.owner.register('service:token', mockToken);

      assert.notOk(this.ability.canList);
    });

    test('it does not permit listing variables when token has a Variables block where paths are without capabilities', function (assert) {
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
                  Variables: {
                    Paths: [
                      { Capabilities: [], PathSpec: '*' },
                      { Capabilities: [], PathSpec: 'foo' },
                      { Capabilities: [], PathSpec: 'foo/bar' },
                    ],
                  },
                },
              ],
            },
          },
        ],
      });

      this.owner.register('service:token', mockToken);

      assert.notOk(this.ability.canList);
    });

    test('it does not permit listing variables when token has no Variables block', function (assert) {
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
                },
              ],
            },
          },
        ],
      });

      this.owner.register('service:token', mockToken);

      assert.notOk(this.ability.canList);
    });

    test('it permits listing variables when token multiple namespaces, only one of which having a Variables block', function (assert) {
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
                  Variables: null,
                },
                {
                  Name: 'nonsense',
                  Capabilities: [],
                  Variables: {
                    Paths: [{ Capabilities: [], PathSpec: '*' }],
                  },
                },
                {
                  Name: 'shenanigans',
                  Capabilities: [],
                  Variables: {
                    Paths: [
                      { Capabilities: ['list'], PathSpec: 'foo/bar/baz' },
                    ],
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
  });

  module('#create', function () {
    test('it does not permit creating variables by default', function (assert) {
      const mockToken = Service.extend({
        aclEnabled: true,
      });

      this.owner.register('service:token', mockToken);

      assert.notOk(this.ability.canWrite);
    });

    test('it permits creating variables when token type is management', function (assert) {
      const mockToken = Service.extend({
        aclEnabled: true,
        selfToken: { type: 'management' },
      });

      this.owner.register('service:token', mockToken);

      assert.ok(this.ability.canWrite);
    });

    test('it permits creating variables when acl is disabled', function (assert) {
      const mockToken = Service.extend({
        aclEnabled: false,
        selfToken: { type: 'client' },
      });

      this.owner.register('service:token', mockToken);

      assert.ok(this.ability.canWrite);
    });

    test('it permits creating variables when token has Variables with write capabilities in its rules', function (assert) {
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
                  Variables: {
                    Paths: [{ Capabilities: ['write'], PathSpec: '*' }],
                  },
                },
              ],
            },
          },
        ],
      });

      this.owner.register('service:token', mockToken);

      assert.ok(this.ability.canWrite);
    });

    test('it handles namespace matching', function (assert) {
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
                  Variables: {
                    Paths: [{ Capabilities: ['list'], PathSpec: 'foo/bar' }],
                  },
                },
                {
                  Name: 'pablo',
                  Capabilities: [],
                  Variables: {
                    Paths: [{ Capabilities: ['write'], PathSpec: 'foo/bar' }],
                  },
                },
              ],
            },
          },
        ],
      });

      this.owner.register('service:token', mockToken);
      this.ability.path = 'foo/bar';
      this.ability.namespace = 'pablo';

      assert.ok(this.ability.canWrite);
    });
  });

  module('#destroy', function () {
    test('it does not permit destroying variables by default', function (assert) {
      const mockToken = Service.extend({
        aclEnabled: true,
      });

      this.owner.register('service:token', mockToken);

      assert.notOk(this.ability.canDestroy);
    });

    test('it permits destroying variables when token type is management', function (assert) {
      const mockToken = Service.extend({
        aclEnabled: true,
        selfToken: { type: 'management' },
      });

      this.owner.register('service:token', mockToken);

      assert.ok(this.ability.canDestroy);
    });

    test('it permits destroying variables when acl is disabled', function (assert) {
      const mockToken = Service.extend({
        aclEnabled: false,
        selfToken: { type: 'client' },
      });

      this.owner.register('service:token', mockToken);

      assert.ok(this.ability.canDestroy);
    });

    test('it permits destroying variables when token has Variables with write capabilities in its rules', function (assert) {
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
                  Variables: {
                    Paths: [{ Capabilities: ['destroy'], PathSpec: '*' }],
                  },
                },
              ],
            },
          },
        ],
      });

      this.owner.register('service:token', mockToken);

      assert.ok(this.ability.canDestroy);
    });

    test('it handles namespace matching', function (assert) {
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
                  Variables: {
                    Paths: [{ Capabilities: ['list'], PathSpec: 'foo/bar' }],
                  },
                },
                {
                  Name: 'pablo',
                  Capabilities: [],
                  Variables: {
                    Paths: [{ Capabilities: ['destroy'], PathSpec: 'foo/bar' }],
                  },
                },
              ],
            },
          },
        ],
      });

      this.owner.register('service:token', mockToken);
      this.ability.path = 'foo/bar';
      this.ability.namespace = 'pablo';

      assert.ok(this.ability.canDestroy);
    });
  });

  module('#read', function () {
    test('it does not permit reading variables by default', function (assert) {
      const mockToken = Service.extend({
        aclEnabled: true,
      });

      this.owner.register('service:token', mockToken);

      assert.notOk(this.ability.canRead);
    });

    test('it permits reading variables when token type is management', function (assert) {
      const mockToken = Service.extend({
        aclEnabled: true,
        selfToken: { type: 'management' },
      });

      this.owner.register('service:token', mockToken);

      assert.ok(this.ability.canRead);
    });

    test('it permits reading variables when acl is disabled', function (assert) {
      const mockToken = Service.extend({
        aclEnabled: false,
        selfToken: { type: 'client' },
      });

      this.owner.register('service:token', mockToken);

      assert.ok(this.ability.canRead);
    });

    test('it permits reading variables when token has Variables with read capabilities in its rules', function (assert) {
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
                  Variables: {
                    Paths: [{ Capabilities: ['read'], PathSpec: '*' }],
                  },
                },
              ],
            },
          },
        ],
      });

      this.owner.register('service:token', mockToken);

      assert.ok(this.ability.canRead);
    });

    test('it handles namespace matching', function (assert) {
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
                  Variables: {
                    Paths: [{ Capabilities: ['list'], PathSpec: 'foo/bar' }],
                  },
                },
                {
                  Name: 'pablo',
                  Capabilities: [],
                  Variables: {
                    Paths: [{ Capabilities: ['read'], PathSpec: 'foo/bar' }],
                  },
                },
              ],
            },
          },
        ],
      });

      this.owner.register('service:token', mockToken);
      this.ability.path = 'foo/bar';
      this.ability.namespace = 'pablo';

      assert.ok(this.ability.canRead);
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
                  Variables: {
                    Paths: [{ Capabilities: ['write'], PathSpec: 'foo' }],
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

    test('returns capabilities for the nearest fuzzy match if no exact match', function (assert) {
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
                  Variables: {
                    Paths: [
                      { Capabilities: ['write'], PathSpec: 'foo/*' },
                      { Capabilities: ['write'], PathSpec: 'foo/bar/*' },
                    ],
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
        'It should return the nearest fuzzy matching path.'
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
                  Variables: {
                    Paths: [{ Capabilities: ['write'], PathSpec: 'foo/*' }],
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
        'It should handle wildcard glob.'
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
                  Variables: {
                    Paths: [
                      { Capabilities: ['write'], PathSpec: '*/bar' },
                      { Capabilities: ['write'], PathSpec: '*/bar/baz' },
                    ],
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
                  Variables: {
                    Paths: [
                      { Capabilities: ['write'], PathSpec: '*/bar' },
                      { Capabilities: ['write'], PathSpec: 'foo/*' },
                    ],
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
                  Variables: {
                    'Path "*"': {
                      Capabilities: ['write'],
                    },
                    'Path "foo"': {
                      Capabilities: ['write'],
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

  module('#_computeLengthDiff', function () {
    test('should return the difference in length between a path and a pattern', function (assert) {
      // arrange
      const path = 'foo';
      const pattern = 'bar';

      // act
      const result = this.ability._computeLengthDiff(pattern, path);

      // assert
      assert.equal(
        result,
        0,
        'it returns the difference in length between path and pattern'
      );
    });

    test('should factor the number of globs into consideration', function (assert) {
      // arrange
      const pattern = 'foo*';
      const path = 'bark';

      // act
      const result = this.ability._computeLengthDiff(pattern, path);

      // assert
      assert.equal(
        result,
        1,
        'it adds the number of globs in the pattern to the difference'
      );
    });
  });

  module('#_smallestDifference', function () {
    test('returns the smallest difference in the list', function (assert) {
      // arrange
      const path = 'foo/bar';
      const matchingPath = 'foo/*';
      const matches = ['*/baz', '*', matchingPath];

      // act
      const result = this.ability._smallestDifference(matches, path);

      // assert
      assert.equal(
        result,
        matchingPath,
        'It should return the smallest difference path.'
      );
    });
  });

  module('#allPaths', function () {
    test('it filters by namespace and shows all matching paths on the namespace', function (assert) {
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
                  Variables: {
                    Paths: [{ Capabilities: ['write'], PathSpec: 'foo' }],
                  },
                },
                {
                  Name: 'bar',
                  Capabilities: [],
                  Variables: {
                    Paths: [
                      { Capabilities: ['read', 'write'], PathSpec: 'foo' },
                    ],
                  },
                },
              ],
            },
          },
        ],
      });

      this.owner.register('service:token', mockToken);
      this.ability.namespace = 'bar';

      const allPaths = this.ability.allPaths;

      assert.deepEqual(
        allPaths,
        [
          {
            capabilities: ['read', 'write'],
            name: 'foo',
          },
        ],
        'It should return the exact path match.'
      );
    });

    test('it matches on default if no namespace is selected', function (assert) {
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
                  Variables: {
                    Paths: [{ Capabilities: ['write'], PathSpec: 'foo' }],
                  },
                },
                {
                  Name: 'bar',
                  Capabilities: [],
                  Variables: {
                    Paths: [
                      { Capabilities: ['read', 'write'], PathSpec: 'foo' },
                    ],
                  },
                },
              ],
            },
          },
        ],
      });

      this.owner.register('service:token', mockToken);
      this.ability.namespace = undefined;

      const allPaths = this.ability.allPaths;

      assert.deepEqual(
        allPaths,
        [
          {
            capabilities: ['write'],
            name: 'foo',
          },
        ],
        'It should return the exact path match.'
      );
    });

    test('it handles globs in namespaces', function (assert) {
      const mockToken = Service.extend({
        aclEnabled: true,
        selfToken: { type: 'client' },
        selfTokenPolicies: [
          {
            rulesJSON: {
              Namespaces: [
                {
                  Name: '*',
                  Capabilities: ['list-jobs', 'alloc-exec', 'read-logs'],
                  Variables: {
                    Paths: [
                      {
                        Capabilities: ['list'],
                        PathSpec: '*',
                      },
                    ],
                  },
                },
                {
                  Name: 'namespace-1',
                  Capabilities: ['list-jobs', 'alloc-exec', 'read-logs'],
                  Variables: {
                    Paths: [
                      {
                        Capabilities: ['list', 'read', 'destroy', 'create'],
                        PathSpec: '*',
                      },
                    ],
                  },
                },
                {
                  Name: 'namespace-2',
                  Capabilities: ['list-jobs', 'alloc-exec', 'read-logs'],
                  Variables: {
                    Paths: [
                      {
                        Capabilities: ['list', 'read', 'destroy', 'create'],
                        PathSpec: 'blue/*',
                      },
                      {
                        Capabilities: ['list', 'read', 'create'],
                        PathSpec: 'nomad/jobs/*',
                      },
                    ],
                  },
                },
              ],
            },
          },
        ],
      });

      this.owner.register('service:token', mockToken);
      this.ability.namespace = 'pablo';

      const allPaths = this.ability.allPaths;

      assert.deepEqual(
        allPaths,
        [
          {
            capabilities: ['list'],
            name: '*',
          },
        ],
        'It should return the glob matching namespace match.'
      );
    });
  });
});
