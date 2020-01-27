import { Ability } from 'ember-can';
import { inject as service } from '@ember/service';
import { computed, get } from '@ember/object';
import { equal, or } from '@ember/object/computed';

export default Ability.extend({
  token: service(),

  // Map abilities to policy options (which are coarse for nodes)
  // instead of specific behaviors.
  canWrite: or('selfTokenIsManagement', 'policiesIncludeNodeWrite'),

  selfTokenIsManagement: equal('token.selfToken.type', 'management'),

  policiesIncludeNodeWrite: computed('token.selfTokenPolicies.[]', function() {
    // For each policy record, extract the Node policy
    const policies = (this.get('token.selfTokenPolicies') || [])
      .toArray()
      .map(policy => get(policy, 'rulesJSON.Node.Policy'))
      .compact();

    // Node write is allowed if any policy allows it
    return policies.some(policy => policy === 'write');
  }),
});
