import AbstractAbility from './abstract';
import { computed } from '@ember/object';
import { or } from '@ember/object/computed';

export default class Recommendation extends AbstractAbility {
  @or('bypassAuthorization', 'selfTokenIsManagement', 'policiesSupportAcceptingOnAnyNamespace')
  canAccept;

  @computed('capabilitiesForAllNamespaces.[]')
  get policiesSupportAcceptingOnAnyNamespace() {
    return this.capabilitiesForAllNamespaces.includes('submit-job');
  }
}
