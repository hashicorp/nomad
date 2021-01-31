/* eslint-disable ember/avoid-leaking-state-in-ember-objects */
import { module, test } from 'qunit';
import { setupTest } from 'ember-qunit';
import Service from '@ember/service';
import setupAbility from 'nomad-ui/tests/helpers/setup-ability';

module('Unit | Ability | job', function(hooks) {
  setupTest(hooks);
  setupAbility('job')(hooks);

  test('it permits job run when ACLs are disabled', function(assert) {
    const mockToken = Service.extend({
      aclEnabled: false,
    });

    this.owner.register('service:token', mockToken);

    assert.ok(this.ability.canRun);
  });

  test('it permits job run for management tokens', function(assert) {
    const mockToken = Service.extend({
      aclEnabled: true,
      selfToken: { type: 'management' },
    });

    this.owner.register('service:token', mockToken);

    assert.ok(this.ability.canRun);
  });

  test('it permits job run for client tokens with a policy that has namespace submit-job', function(assert) {
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
                Capabilities: ['submit-job'],
              },
            ],
          },
        },
      ],
    });

    this.owner.register('service:system', mockSystem);
    this.owner.register('service:token', mockToken);

    assert.ok(this.ability.canRun);
  });

  test('it permits job run for client tokens with a policy that has default namespace submit-job and no capabilities for active namespace', function(assert) {
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
                Capabilities: ['submit-job'],
              },
            ],
          },
        },
      ],
    });

    this.owner.register('service:system', mockSystem);
    this.owner.register('service:token', mockToken);

    assert.ok(this.ability.canRun);
  });

  test('it blocks job run for client tokens with a policy that has no submit-job capability', function(assert) {
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

    assert.notOk(this.ability.canRun);
  });

  test('job scale requires a client token with the submit-job or scale-job capability', function(assert) {
    const makePolicies = (namespace, ...capabilities) => [
      {
        rulesJSON: {
          Namespaces: [
            {
              Name: namespace,
              Capabilities: capabilities,
            },
          ],
        },
      },
    ];

    const mockSystem = Service.extend({
      aclEnabled: true,
      activeNamespace: {
        name: 'aNamespace',
      },
    });

    const mockToken = Service.extend({
      aclEnabled: true,
      selfToken: { type: 'client' },
      selfTokenPolicies: makePolicies('aNamespace'),
    });

    this.owner.register('service:system', mockSystem);
    this.owner.register('service:token', mockToken);
    const tokenService = this.owner.lookup('service:token');

    assert.notOk(this.ability.canScale);

    tokenService.set('selfTokenPolicies', makePolicies('aNamespace', 'scale-job'));
    assert.ok(this.ability.canScale);

    tokenService.set('selfTokenPolicies', makePolicies('aNamespace', 'submit-job'));
    assert.ok(this.ability.canScale);

    tokenService.set('selfTokenPolicies', makePolicies('bNamespace', 'scale-job'));
    assert.notOk(this.ability.canScale);
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
                Capabilities: ['submit-job'],
              },
              {
                Name: 'production-api',
                Capabilities: ['submit-job'],
              },
              {
                Name: 'production-web',
                Capabilities: [],
              },
              {
                Name: '*-suffixed',
                Capabilities: ['submit-job'],
              },
              {
                Name: '*-more-suffixed',
                Capabilities: [],
              },
              {
                Name: '*-abc-*',
                Capabilities: ['submit-job'],
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
    assert.notOk(this.ability.canRun);

    systemService.set('activeNamespace.name', 'production-api');
    assert.ok(this.ability.canRun);

    systemService.set('activeNamespace.name', 'production-other');
    assert.ok(this.ability.canRun);

    systemService.set('activeNamespace.name', 'something-suffixed');
    assert.ok(this.ability.canRun);

    systemService.set('activeNamespace.name', 'something-more-suffixed');
    assert.notOk(
      this.ability.canRun,
      'expected the namespace with the greatest number of matched characters to be chosen'
    );

    systemService.set('activeNamespace.name', '000-abc-999');
    assert.ok(this.ability.canRun, 'expected to be able to match against more than one wildcard');
  });
});
