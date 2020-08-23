import AbstractAbility from './abstract';
import { computed, get } from '@ember/object';
import { or } from '@ember/object/computed';

export default class Allocation extends AbstractAbility {
  @or('bypassAuthorization', 'selfTokenIsManagement', 'policiesSupportExec')
  canExec;

  @computed('rulesForActiveNamespace.@each.capabilities')
  get policiesSupportExec() {
    return this.rulesForActiveNamespace.some(rules => {
      let capabilities = get(rules, 'Capabilities') || [];
      return capabilities.includes('alloc-exec');
    });
  }
}
