import AbstractAbility from './abstract';
import { computed, get } from '@ember/object';
import { or } from '@ember/object/computed';

export default AbstractAbility.extend({
  canRun: or('bypassAuthorization', 'selfTokenIsManagement', 'policiesSupportRunning'),

  policiesSupportRunning: computed('rulesForActiveNamespace.@each.capabilities', function() {
    return this.rulesForActiveNamespace.some(rules => {
      let capabilities = get(rules, 'Capabilities') || [];
      return capabilities.includes('submit-job');
    });
  }),
});
