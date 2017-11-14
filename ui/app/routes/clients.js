import Ember from 'ember';
import WithForbiddenState from 'nomad-ui/mixins/with-forbidden-state';
import notifyForbidden from 'nomad-ui/utils/notify-forbidden';

const { Route, inject, RSVP } = Ember;

export default Route.extend(WithForbiddenState, {
  store: inject.service(),
  system: inject.service(),

  beforeModel() {
    return this.get('system.leader');
  },

  model() {
    return RSVP.hash({
      nodes: this.get('store').findAll('node'),
      agents: this.get('store').findAll('agent'),
    }).catch(notifyForbidden(this));
  },
});
