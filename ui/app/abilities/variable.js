import AbstractAbility from './abstract';
import { computed, get } from '@ember/object';
import { or } from '@ember/object/computed';

export default class extends AbstractAbility {
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

  @computed('rulesForNamespace.@each.capabilities') // TODO:  edit computed property to be SecureVariables.Path "DYNAMIC PATH"
  get policiesSupportVariableCreation() {
    return this.rulesForNamespace.some((rules) => {
      const keyName = `SecureVariables.Path "*".Capabilities`; // TODO:  add ability to edit path, however computed properties can't take parameters
      const capabilities = get(rules, keyName) || [];
      return capabilities.includes('create');
    });
  }
}
