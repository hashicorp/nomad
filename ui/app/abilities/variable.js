import { computed, get } from '@ember/object';
import { or } from '@ember/object/computed';
import AbstractAbility from './abstract';

export default class Variable extends AbstractAbility {
  // Pass in a namespace to `can` or `cannot` calls to override
  // https://github.com/minutebase/ember-can#additional-attributes
  path = '*';

  get _path() {
    if (!this.path) return '*';
    return this.path;
  }

  @or(
    'bypassAuthorization',
    'selfTokenIsManagement',
    'policiesSupportVariableView'
  )
  canList;

  @or(
    'bypassAuthorization',
    'selfTokenIsManagement',
    'policiesSupportVariableCreation'
  )
  canCreate;

  @computed('rulesForNamespace.@each.capabilities')
  get policiesSupportVariableView() {
    return this.rulesForNamespace.some((rules) => {
      return get(rules, 'SecureVariables');
    });
  }

  @computed('rulesForNamespace.@each.capabilities', 'path')
  get policiesSupportVariableCreation() {
    return this.rulesForNamespace.some((rules) => {
      const keyName = `SecureVariables.Path "${this.path}".Capabilities`;
      const capabilities = get(rules, keyName) || [];
      return capabilities.includes('create');
    });
  }

  _nearestMatchingPath(path) {
    const formattedPathKey = `Path "${path}"`;
    const pathNames = Object.keys(this.allPaths);

    if (pathNames.includes(formattedPathKey)) return path;
  }

  @computed('token.selfTokenPolicies.[]')
  get allPaths() {
    return get(this, 'token.selfTokenPolicies')
      .toArray()
      .reduce((paths, policy) => {
        const [variables] = get(policy, 'rulesJSON.Namespaces').mapBy(
          'SecureVariables'
        );
        paths = { ...paths, ...variables };
        return paths;
      }, {});
  }
}
