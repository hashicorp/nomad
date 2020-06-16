import AbstractAbility from './abstract';
import { computed, get } from '@ember/object';
import { or } from '@ember/object/computed';

export default class Job extends AbstractAbility {
  @or('bypassAuthorization', 'selfTokenIsManagement', 'policiesSupportRunning')
  canRun;

  @computed('rulesForActiveNamespace.@each.capabilities')
  get policiesSupportRunning() {
    return this.rulesForActiveNamespace.some(rules => {
      let capabilities = get(rules, 'Capabilities') || [];
      return capabilities.includes('submit-job');
    });
  }
}
