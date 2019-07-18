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

        const matchingNamespace = this._findMatchingNamespace(policyNamespaces, activeNamespace);

        if (matchingNamespace) {
          rules.push(policyNamespaces[matchingNamespace]);
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

  _findMatchingNamespace(policyNamespaces, activeNamespace) {
    if (policyNamespaces[activeNamespace]) {
      return activeNamespace;
    }

    const namespaceNames = Object.keys(policyNamespaces);
    const globNamespaceNames = namespaceNames.filter(namespaceName => namespaceName.includes('*'));

    const matchingNamespaceName = globNamespaceNames.find(namespaceName => {
      // TODO what kind of protection/sanitisation is needed here, if any?
      return activeNamespace.match(new RegExp(namespaceName));
    });

    if (matchingNamespaceName) {
      return matchingNamespaceName;
    } else if (policyNamespaces.default) {
      return 'default';
    }
  },
});
