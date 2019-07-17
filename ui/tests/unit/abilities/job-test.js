import { module, skip, test } from 'qunit';
import { setupTest } from 'ember-qunit';
import Service from '@ember/service';

module('Unit | Ability | job run FIXME just for ease of filtering', function(hooks) {
  setupTest(hooks);

  test('it permits job run for management tokens', function(assert) {
    const mockToken = Service.extend({
      selfToken: { type: 'management' },
    });

    this.owner.register('service:token', mockToken);

    const jobAbility = this.owner.lookup('ability:job');
    assert.ok(jobAbility.canRun);
  });

  test('it permits job run for client tokens with a policy that has namespace write', function(assert) {
    const mockSystem = Service.extend({
      activeNamespace: {
        name: 'aNamespace',
      },
    });

    const mockToken = Service.extend({
      selfToken: { type: 'client' },
      selfTokenPolicies: [
        {
          rulesJson: {
            namespace: {
              aNamespace: {
                policy: 'write',
              },
            },
          },
        },
      ],
    });

    this.owner.register('service:system', mockSystem);
    this.owner.register('service:token', mockToken);

    const jobAbility = this.owner.lookup('ability:job');
    assert.ok(jobAbility.canRun);
  });

  // TODO is this true, that a more-permissive default wins?
  skip('it permits job run for client tokens with a policy that has default namespace write', function(assert) {
    const mockSystem = Service.extend({
      activeNamespace: {
        name: 'aNamespace',
      },
    });

    const mockToken = Service.extend({
      selfToken: { type: 'client' },
      selfTokenPolicies: [
        {
          rulesJson: {
            namespace: {
              aNamespace: {
                policy: 'read',
              },
              default: {
                policy: 'write',
              },
            },
          },
        },
      ],
    });

    this.owner.register('service:system', mockSystem);
    this.owner.register('service:token', mockToken);

    const jobAbility = this.owner.lookup('ability:job');
    assert.ok(jobAbility.canRun);
  });

  test('it blocks job run for client tokens with a policy that has namespace read', function(assert) {
    const mockSystem = Service.extend({
      activeNamespace: {
        name: 'aNamespace',
      },
    });

    const mockToken = Service.extend({
      selfToken: { type: 'client' },
      selfTokenPolicies: [
        {
          rulesJson: {
            namespace: {
              aNamespace: {
                policy: 'read',
              },
            },
          },
        },
      ],
    });

    this.owner.register('service:system', mockSystem);
    this.owner.register('service:token', mockToken);

    const jobAbility = this.owner.lookup('ability:job');
    assert.notOk(jobAbility.canRun);
  });
});
