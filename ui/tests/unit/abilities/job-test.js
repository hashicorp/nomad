import { module, test } from 'qunit';
import { setupTest } from 'ember-qunit';
import Service from '@ember/service';

module('Unit | Ability | job', function(hooks) {
  setupTest(hooks);

  test('it permits job run for management tokens', function(assert) {
    const mockToken = Service.extend({
      selfToken: { type: 'management' },
    });

    this.owner.register('service:token', mockToken);

    const jobAbility = this.owner.lookup('ability:job');
    assert.ok(jobAbility.canRun);
  });

  test('it permits job run for client tokens with a policy that has namespace submit-job', function(assert) {
    const mockSystem = Service.extend({
      activeNamespace: {
        name: 'aNamespace',
      },
    });

    const mockToken = Service.extend({
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

    const jobAbility = this.owner.lookup('ability:job');
    assert.ok(jobAbility.canRun);
  });

  test('it permits job run for client tokens with a policy that has default namespace submit-job and no capabilities for active namespace', function(assert) {
    const mockSystem = Service.extend({
      activeNamespace: {
        name: 'anotherNamespace',
      },
    });

    const mockToken = Service.extend({
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

    const jobAbility = this.owner.lookup('ability:job');
    assert.ok(jobAbility.canRun);
  });

  test('it blocks job run for client tokens with a policy that has no submit-job capability', function(assert) {
    const mockSystem = Service.extend({
      activeNamespace: {
        name: 'aNamespace',
      },
    });

    const mockToken = Service.extend({
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

    const jobAbility = this.owner.lookup('ability:job');
    assert.notOk(jobAbility.canRun);
  });

  test('it handles globs in namespace names', function(assert) {
    const mockSystem = Service.extend({
      activeNamespace: {
        name: 'aNamespace',
      },
    });

    const mockToken = Service.extend({
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

    const jobAbility = this.owner.lookup('ability:job');
    const systemService = this.owner.lookup('service:system');

    systemService.set('activeNamespace.name', 'production-web');
    assert.notOk(jobAbility.canRun);

    systemService.set('activeNamespace.name', 'production-api');
    assert.ok(jobAbility.canRun);

    systemService.set('activeNamespace.name', 'production-other');
    assert.ok(jobAbility.canRun);

    systemService.set('activeNamespace.name', 'something-suffixed');
    assert.ok(jobAbility.canRun);

    systemService.set('activeNamespace.name', 'something-more-suffixed');
    assert.notOk(
      jobAbility.canRun,
      'expected the namespace with the greatest number of matched characters to be chosen'
    );

    systemService.set('activeNamespace.name', '000-abc-999');
    assert.ok(jobAbility.canRun, 'expected to be able to match against more than one wildcard');
  });
});
