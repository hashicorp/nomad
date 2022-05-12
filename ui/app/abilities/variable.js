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

  // @computed('rulesForNamespace.@each.capabilities')
  // get policiesSupportVariableRead() {
  //   return this.namespaceIncludesCapability('submit-job');
  // }

  @computed('rulesForNamespace.@each.capabilities')
  get policiesSupportVariableRead() {
    console.log(
      'rules4?',
      this.rulesForNamespace,
      this.capabilitiesForAllNamespaces
    );
    return this.rulesForNamespace.some((rules) => {
      console.log('computing', rules, get(rules, 'Capabilities'));
      let capabilities = get(rules, 'Capabilities') || [];
      return capabilities.includes('alloc-exec');
    });
  }
}
