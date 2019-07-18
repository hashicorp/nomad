import { Ability } from 'ember-can';
import { inject as service } from '@ember/service';
import { computed, getWithDefault } from '@ember/object';
import { equal, or } from '@ember/object/computed';

export default Ability.extend({
  system: service(),
  token: service(),

  canRun: or('selfTokenIsManagement', 'policiesSupportRunning'),

  selfTokenIsManagement: equal('token.selfToken.type', 'management'),

  activeNamespace: computed('system.activeNamespace.name', function() {
    return this.get('system.activeNamespace.name') || 'default';
  }),

  rulesForActiveNamespace: computed(
    'activeNamespace',
    'token.selfTokenPolicies.@each.namespace' /* FIXME not quite */,
    function() {
      const activeNamespace = this.activeNamespace;

      return (this.get('token.selfTokenPolicies') || []).toArray().reduce((rules, policy) => {
        const policyNamespaces = getWithDefault(policy, 'rulesJson.namespace', {});

        if (policyNamespaces[activeNamespace]) {
          rules.push(policyNamespaces[activeNamespace]);
        } else if (policyNamespaces.default) {
          rules.push(policyNamespaces.default);
        }

        return rules;
      }, []);
    }
  ),

  policiesSupportRunning: computed(
    'rulesForActiveNamespace.@each.policy',
    'rulesForActiveNamespace.@each.capabilities',
    function() {
      return this.rulesForActiveNamespace.some(rules => {
        const policy = rules.policy;
        const capabilities = getWithDefault(rules, 'capabilities', []);

        return policy == 'write' || capabilities.includes('submit-job');
      });
    }
  ),
});
