import AbstractAbility from './abstract';
import { computed } from '@ember/object';
import { or } from '@ember/object/computed';

export default class Job extends AbstractAbility {
  @or('bypassAuthorization', 'selfTokenIsManagement', 'policiesSupportRunning')
  canRun;

  @or(
    'bypassAuthorization',
    'selfTokenIsManagement',
    'policiesSupportRunning',
    'policiesSupportScaling'
  )
  canScale;

  // TODO: A person can also see all jobs if their token grants read access to all namespaces,
  // but given the complexity of namespaces and policy precedence, there isn't a good quick way
  // to confirm this.
  @or('bypassAuthorization', 'selfTokenIsManagement')
  canListAll;

  @computed('rulesForActiveNamespace.@each.capabilities')
  get policiesSupportRunning() {
    return this.activeNamespaceIncludesCapability('submit-job');
  }

  @computed('rulesForActiveNamespace.@each.capabilities')
  get policiesSupportScaling() {
    return this.activeNamespaceIncludesCapability('scale-job');
  }
}
