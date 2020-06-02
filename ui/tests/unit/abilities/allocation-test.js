/* eslint-disable ember/avoid-leaking-state-in-ember-objects */
import { module, test } from 'qunit';
import { setupTest } from 'ember-qunit';
import Service from '@ember/service';
import setupAbility from 'nomad-ui/tests/helpers/setup-ability';

module('Unit | Ability | allocation', function(hooks) {
  setupTest(hooks);
  setupAbility('allocation')(hooks);

  test('it permits alloc exec when ACLs are disabled', function(assert) {
    const mockToken = Service.extend({
      aclEnabled: false,
    });

    this.owner.register('service:token', mockToken);

    assert.ok(this.ability.canExec);
  });

  test('it permits alloc exec for management tokens', function(assert) {
    const mockToken = Service.extend({
      aclEnabled: true,
      selfToken: { type: 'management' },
    });

    this.owner.register('service:token', mockToken);

    assert.ok(this.ability.canExec);
  });

  test('it permits alloc exec for client tokens with a policy that has namespace alloc-exec', function(assert) {
    const mockSystem = Service.extend({
      aclEnabled: true,
      activeNamespace: {
        name: 'aNamespace',
      },
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
                Capabilities: ['alloc-exec'],
              },
            ],
          },
        },
      ],
    });

    this.owner.register('service:system', mockSystem);
    this.owner.register('service:token', mockToken);

    assert.ok(this.ability.canExec);
  });

  test('it permits alloc exec for client tokens with a policy that has default namespace alloc-exec and no capabilities for active namespace', function(assert) {
    const mockSystem = Service.extend({
      aclEnabled: true,
      activeNamespace: {
        name: 'anotherNamespace',
      },
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
                Name: 'default',
                Capabilities: ['alloc-exec'],
              },
            ],
          },
        },
      ],
    });

    this.owner.register('service:system', mockSystem);
    this.owner.register('service:token', mockToken);

    assert.ok(this.ability.canExec);
  });

  test('it blocks alloc exec for client tokens with a policy that has no alloc-exec capability', function(assert) {
    const mockSystem = Service.extend({
      aclEnabled: true,
      activeNamespace: {
        name: 'aNamespace',
      },
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
                Capabilities: ['list-jobs'],
              },
            ],
          },
        },
      ],
    });

    this.owner.register('service:system', mockSystem);
    this.owner.register('service:token', mockToken);

    assert.notOk(this.ability.canExec);
  });

  test('it handles globs in namespace names', function(assert) {
    const mockSystem = Service.extend({
      aclEnabled: true,
      activeNamespace: {
        name: 'aNamespace',
      },
    });

    const mockToken = Service.extend({
      aclEnabled: true,
      selfToken: { type: 'client' },
      selfTokenPolicies: [
        {
          rulesJSON: {
            Namespaces: [
              {
                Name: 'production-*',
                Capabilities: ['alloc-exec'],
              },
              {
                Name: 'production-api',
                Capabilities: ['alloc-exec'],
              },
              {
                Name: 'production-web',
                Capabilities: [],
              },
              {
                Name: '*-suffixed',
                Capabilities: ['alloc-exec'],
              },
              {
                Name: '*-more-suffixed',
                Capabilities: [],
              },
              {
                Name: '*-abc-*',
                Capabilities: ['alloc-exec'],
              },
            ],
          },
        },
      ],
    });

    this.owner.register('service:system', mockSystem);
    this.owner.register('service:token', mockToken);

    const systemService = this.owner.lookup('service:system');

    systemService.set('activeNamespace.name', 'production-web');
    assert.notOk(this.ability.canExec);

    systemService.set('activeNamespace.name', 'production-api');
    assert.ok(this.ability.canExec);

    systemService.set('activeNamespace.name', 'production-other');
    assert.ok(this.ability.canExec);

    systemService.set('activeNamespace.name', 'something-suffixed');
    assert.ok(this.ability.canExec);

    systemService.set('activeNamespace.name', 'something-more-suffixed');
    assert.notOk(
      this.ability.canExec,
      'expected the namespace with the greatest number of matched characters to be chosen'
    );

    systemService.set('activeNamespace.name', '000-abc-999');
    assert.ok(this.ability.canExec, 'expected to be able to match against more than one wildcard');
  });
});
