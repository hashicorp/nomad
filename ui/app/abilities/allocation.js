import AbstractAbility from './abstract';
import { computed, get } from '@ember/object';
import { or } from '@ember/object/computed';

export default AbstractAbility.extend({
  canExec: or('bypassAuthorization', 'selfTokenIsManagement', 'policiesSupportExec'),

  policiesSupportExec: computed('rulesForActiveNamespace.@each.capabilities', function() {
    return this.rulesForActiveNamespace.some(rules => {
      let capabilities = get(rules, 'Capabilities') || [];
      return capabilities.includes('alloc-exec');
    });
  }),
});
