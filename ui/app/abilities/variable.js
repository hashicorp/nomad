import { computed, get } from '@ember/object';
import { or } from '@ember/object/computed';
import { inject as service } from '@ember/service';
import AbstractAbility from './abstract';

export default class Variable extends AbstractAbility {
  @service router;

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

  @computed(
    'rulesForNamespace.@each.capabilities',
    'router.currentRoute.params.absolutePath'
  )
  get policiesSupportVariableCreation() {
    const path = get(this, 'router.currentRoute.params.absolutePath') || '*';
    return this.rulesForNamespace.some((rules) => {
      const keyName = `SecureVariables.Path "${path}".Capabilities`;
      const capabilities = get(rules, keyName) || [];
      return capabilities.includes('create');
    });
  }
}
