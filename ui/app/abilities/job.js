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

  @or('bypassAuthorization', 'selfTokenIsManagement', 'policiesSupportDispatching')
  canDispatch;

  @computed('rulesForNamespace.@each.capabilities')
  get policiesSupportRunning() {
    return this.namespaceIncludesCapability('submit-job');
  }

  @computed('rulesForNamespace.@each.capabilities')
  get policiesSupportScaling() {
    return this.namespaceIncludesCapability('scale-job');
  }

  @computed('rulesForNamespace.@each.capabilities')
  get policiesSupportDispatching() {
    return this.namespaceIncludesCapability('dispatch-job');
  }
}
