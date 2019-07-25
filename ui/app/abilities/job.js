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
    'token.selfTokenPolicies.@each.Namespaces' /* FIXME not quite */,
    function() {
      const activeNamespace = this.activeNamespace;

      return (this.get('token.selfTokenPolicies') || []).toArray().reduce((rules, policy) => {
        const policyNamespaces = getWithDefault(policy, 'rulesJSON.Namespaces', []);

        const matchingNamespace = this._findMatchingNamespace(policyNamespaces, activeNamespace);

        if (matchingNamespace) {
          rules.push(policyNamespaces.find(namespace => namespace.Name === matchingNamespace));
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
        // TODO given that the API returns a fully-expanded set of rules,
        // where just a policy word turns into an array of capabilities,
        // maybe checking capabilities is the only necessity?
        const policy = rules.Policy;
        const capabilities = getWithDefault(rules, 'Capabilities', []);

        return policy == 'write' || capabilities.includes('submit-job');
      });
    }
  ),

  _findMatchingNamespace(policyNamespaces, activeNamespace) {
    const namespaceNames = policyNamespaces.mapBy('Name');

    if (namespaceNames.includes(activeNamespace)) {
      return activeNamespace;
    }

    const globNamespaceNames = namespaceNames.filter(namespaceName => namespaceName.includes('*'));

    const matchingNamespaceName = globNamespaceNames.reduce(
      (mostMatching, namespaceName) => {
        // TODO what kind of protection/sanitisation is needed here, if any?
        // and, is * the only matchable character?
        const namespaceNameRegExp = new RegExp(namespaceName.replace(/\*/g, '.*'));
        const characterDifference = activeNamespace.length - namespaceName.length;

        if (
          characterDifference < mostMatching.mostMatchingCharacterDifference &&
          activeNamespace.match(namespaceNameRegExp)
        ) {
          return {
            mostMatchingNamespaceName: namespaceName,
            mostMatchingCharacterDifference: characterDifference,
          };
        } else {
          return mostMatching;
        }
      },
      { mostMatchingNamespaceName: null, mostMatchingCharacterDifference: Number.MAX_SAFE_INTEGER }
    ).mostMatchingNamespaceName;

    if (matchingNamespaceName) {
      return matchingNamespaceName;
    } else if (namespaceNames.includes('default')) {
      return 'default';
    }
  },
});
