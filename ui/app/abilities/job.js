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

  @computed('rulesForActiveNamespace.@each.capabilities')
  get policiesSupportRunning() {
    return this.activeNamespaceIncludesCapability('submit-job');
  }

  @computed('rulesForActiveNamespace.@each.capabilities')
  get policiesSupportScaling() {
    return this.activeNamespaceIncludesCapability('scale-job');
  }
}
