/* eslint-disable ember/avoid-leaking-state-in-ember-objects */
import { module, test } from 'qunit';
import { setupTest } from 'ember-qunit';
import Service from '@ember/service';
import setupAbility from 'nomad-ui/tests/helpers/setup-ability';

module('Unit | Ability | recommendation', function(hooks) {
  setupTest(hooks);
  setupAbility('recommendation')(hooks);

  test('it permits accepting recommendations when ACLs are disabled', function(assert) {
    const mockToken = Service.extend({
      aclEnabled: false,
    });

    this.owner.register('service:token', mockToken);

    assert.ok(this.ability.canAccept);
  });

  test('it permits accepting recommendations for client tokens where any namespace has submit-job capabilities', function(assert) {
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
                Name: 'bNamespace',
                Capabilities: ['submit-job'],
              },
            ],
          },
        },
      ],
    });

    this.owner.register('service:system', mockSystem);
    this.owner.register('service:token', mockToken);

    assert.ok(this.ability.canAccept);
  });
});
