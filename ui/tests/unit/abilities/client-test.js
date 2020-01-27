import { module, test } from 'qunit';
import { setupTest } from 'ember-qunit';
import Service from '@ember/service';

function setupAbility(ability, hooks) {
  hooks.beforeEach(function() {
    this.ability = this.owner.lookup(`ability:${ability}`);
  });

  hooks.afterEach(function() {
    delete this.ability;
  });
}

module('Unit | Ability | client', function(hooks) {
  setupTest(hooks);
  setupAbility('client', hooks);

  test('it permits client write for management tokens', function(assert) {
    const mockToken = Service.extend({
      selfToken: { type: 'management' },
    });
    this.owner.register('service:token', mockToken);

    assert.ok(this.ability.canWrite);
  });

  test('it permits client write for tokens with a policy that has node-write', function(assert) {
    const mockToken = Service.extend({
      selfToken: { type: 'client' },
      selfTokenPolicies: [
        {
          rulesJSON: {
            Node: {
              Policy: 'write',
            },
          },
        },
      ],
    });
    this.owner.register('service:token', mockToken);

    assert.ok(this.ability.canWrite);
  });

  test('it permits client write for tokens with a policy that allows write and another policy that disallows it', function(assert) {
    const mockToken = Service.extend({
      selfToken: { type: 'client' },
      selfTokenPolicies: [
        {
          rulesJSON: {
            Node: {
              Policy: 'write',
            },
          },
        },
        {
          rulesJSON: {
            Node: {
              Policy: 'read',
            },
          },
        },
      ],
    });
    this.owner.register('service:token', mockToken);

    assert.ok(this.ability.canWrite);
  });

  test('it blocks client write for tokens with a policy that does not allow node-write', function(assert) {
    const mockToken = Service.extend({
      selfToken: { type: 'client' },
      selfTokenPolicies: [
        {
          rulesJSON: {
            Node: {
              Policy: 'read',
            },
          },
        },
      ],
    });
    this.owner.register('service:token', mockToken);

    assert.notOk(this.ability.canWrite);
  });
});
