import AbstractAbility from './abstract';
import { computed, get } from '@ember/object';
import { or } from '@ember/object/computed';

export default class extends AbstractAbility {
  @or(
    'bypassAuthorization',
    'selfTokenIsManagement',
    'policiesSupportVariableRead'
  )
  canList;

  @computed('rulesForNamespace.@each.capabilities')
  get policiesSupportVariableView() {
    return this.rulesForNamespace.some((rules) => {
      return get(rules, 'SecureVariables');
    });
  }
}
