import { Ability } from 'ember-can';
import { inject as service } from '@ember/service';
import { computed, get } from '@ember/object';
import { equal, not } from '@ember/object/computed';
import classic from 'ember-classic-decorator';

@classic
export default class Abstract extends Ability {
  @service system;
  @service token;

  @not('token.aclEnabled') bypassAuthorization;
  @equal('token.selfToken.type', 'management') selfTokenIsManagement;

  @computed('system.activeNamespace.name')
  get activeNamespace() {
    return this.get('system.activeNamespace.name') || 'default';
  }

  @computed('activeNamespace', 'token.selfTokenPolicies.[]')
  get rulesForActiveNamespace() {
    let activeNamespace = this.activeNamespace;

    return (this.get('token.selfTokenPolicies') || []).toArray().reduce((rules, policy) => {
      let policyNamespaces = get(policy, 'rulesJSON.Namespaces') || [];

      let matchingNamespace = this._findMatchingNamespace(policyNamespaces, activeNamespace);

      if (matchingNamespace) {
        rules.push(policyNamespaces.find(namespace => namespace.Name === matchingNamespace));
      }

      return rules;
    }, []);
  }

  @computed('token.selfTokenPolicies.[]')
  get capabilitiesForAllNamespaces() {
    return (this.get('token.selfTokenPolicies') || [])
      .toArray()
      .reduce((allCapabilities, policy) => {
        (get(policy, 'rulesJSON.Namespaces') || []).forEach(({ Capabilities }) => {
          allCapabilities = allCapabilities.concat(Capabilities);
        });
        return allCapabilities;
      }, []);
  }

  activeNamespaceIncludesCapability(capability) {
    return this.rulesForActiveNamespace.some(rules => {
      let capabilities = get(rules, 'Capabilities') || [];
      return capabilities.includes(capability);
    });
  }

  // Chooses the closest namespace as described at the bottom here:
  // https://www.nomadproject.io/guides/security/acl.html#namespace-rules
  _findMatchingNamespace(policyNamespaces, activeNamespace) {
    let namespaceNames = policyNamespaces.mapBy('Name');

    if (namespaceNames.includes(activeNamespace)) {
      return activeNamespace;
    }

    let globNamespaceNames = namespaceNames.filter(namespaceName => namespaceName.includes('*'));

    let matchingNamespaceName = globNamespaceNames.reduce(
      (mostMatching, namespaceName) => {
        // Convert * wildcards to .* for regex matching
        let namespaceNameRegExp = new RegExp(namespaceName.replace(/\*/g, '.*'));
        let characterDifference = activeNamespace.length - namespaceName.length;

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
  }
}
