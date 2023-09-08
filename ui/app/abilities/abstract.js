/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

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

  // Pass in a namespace to `can` or `cannot` calls to override
  // https://github.com/minutebase/ember-can#additional-attributes
  namespace = 'default';

  get _namespace() {
    if (!this.namespace) return 'default';
    if (typeof this.namespace === 'string') return this.namespace;
    return get(this.namespace, 'name');
  }

  @computed('_namespace', 'token.selfTokenPolicies.[]')
  get rulesForNamespace() {
    let namespace = this._namespace;

    return (this.get('token.selfTokenPolicies') || [])
      .toArray()
      .reduce((rules, policy) => {
        let policyNamespaces = get(policy, 'rulesJSON.Namespaces') || [];

        let matchingNamespace = this._findMatchingNamespace(
          policyNamespaces,
          namespace
        );

        if (matchingNamespace) {
          rules.push(
            policyNamespaces.find(
              (namespace) => namespace.Name === matchingNamespace
            )
          );
        }

        return rules;
      }, []);
  }

  @computed('token.selfTokenPolicies.[]')
  get capabilitiesForAllNamespaces() {
    return (this.get('token.selfTokenPolicies') || [])
      .toArray()
      .reduce((allCapabilities, policy) => {
        (get(policy, 'rulesJSON.Namespaces') || []).forEach(
          ({ Capabilities }) => {
            allCapabilities = allCapabilities.concat(Capabilities);
          }
        );
        return allCapabilities;
      }, []);
  }

  namespaceIncludesCapability(capability) {
    return this.rulesForNamespace.some((rules) => {
      let capabilities = get(rules, 'Capabilities') || [];
      return capabilities.includes(capability);
    });
  }

  @computed('system.features.[]')
  get features() {
    return this.system.features;
  }

  featureIsPresent(featureName) {
    return this.features.includes(featureName);
  }

  // Chooses the closest namespace as described at the bottom here:
  // https://learn.hashicorp.com/tutorials/nomad/access-control-policies?in=nomad/access-control#namespace-rules
  _findMatchingNamespace(policyNamespaces, namespace) {
    let namespaceNames = policyNamespaces.mapBy('Name');

    if (namespaceNames.includes(namespace)) {
      return namespace;
    }

    let globNamespaceNames = namespaceNames.filter((namespaceName) =>
      namespaceName.includes('*')
    );

    let matchingNamespaceName = globNamespaceNames.reduce(
      (mostMatching, namespaceName) => {
        // Convert * wildcards to .* for regex matching
        let namespaceNameRegExp = new RegExp(
          namespaceName.replace(/\*/g, '.*')
        );
        let characterDifference = namespace.length - namespaceName.length;

        if (
          characterDifference < mostMatching.mostMatchingCharacterDifference &&
          namespace.match(namespaceNameRegExp)
        ) {
          return {
            mostMatchingNamespaceName: namespaceName,
            mostMatchingCharacterDifference: characterDifference,
          };
        } else {
          return mostMatching;
        }
      },
      {
        mostMatchingNamespaceName: null,
        mostMatchingCharacterDifference: Number.MAX_SAFE_INTEGER,
      }
    ).mostMatchingNamespaceName;

    if (matchingNamespaceName) {
      return matchingNamespaceName;
    } else if (namespaceNames.includes('default')) {
      return 'default';
    }
  }
}
