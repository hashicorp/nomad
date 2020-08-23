import AbstractAbility from './abstract';
import { computed, get } from '@ember/object';
import { or } from '@ember/object/computed';
import classic from 'ember-classic-decorator';

@classic
export default class Client extends AbstractAbility {
  // Map abilities to policy options (which are coarse for nodes)
  // instead of specific behaviors.
  @or('bypassAuthorization', 'selfTokenIsManagement', 'policiesIncludeNodeWrite')
  canWrite;

  @computed('token.selfTokenPolicies.[]')
  get policiesIncludeNodeWrite() {
    // For each policy record, extract the Node policy
    const policies = (this.get('token.selfTokenPolicies') || [])
      .toArray()
      .map(policy => get(policy, 'rulesJSON.Node.Policy'))
      .compact();

    // Node write is allowed if any policy allows it
    return policies.some(policy => policy === 'write');
  }
}
