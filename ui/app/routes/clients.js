import { inject as service } from '@ember/service';
import Route from '@ember/routing/route';
import RSVP from 'rsvp';
import WithForbiddenState from 'nomad-ui/mixins/with-forbidden-state';
import notifyForbidden from 'nomad-ui/utils/notify-forbidden';

export default Route.extend(WithForbiddenState, {
  store: service(),
  system: service(),

  breadcrumbs: [
    {
      label: 'Clients',
      args: ['clients.index'],
    },
  ],

  beforeModel() {
    return this.get('system.leader');
  },

  model() {
    return RSVP.hash({
      nodes: this.store.findAll('node'),
      agents: this.store.findAll('agent'),
    }).catch(notifyForbidden(this));
  },
});
