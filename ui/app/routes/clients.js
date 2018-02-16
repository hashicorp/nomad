import { inject as service } from '@ember/service';
import Route from '@ember/routing/route';
import RSVP from 'rsvp';
import WithForbiddenState from 'nomad-ui/mixins/with-forbidden-state';
import notifyForbidden from 'nomad-ui/utils/notify-forbidden';
import { watchAll } from 'nomad-ui/utils/properties/watch';

export default Route.extend(WithForbiddenState, {
  store: service(),
  system: service(),

  beforeModel() {
    return this.get('system.leader');
  },

  model() {
    return RSVP.hash({
      nodes: this.get('store').findAll('node'),
      agents: this.get('store').findAll('agent'),
    }).catch(notifyForbidden(this));
  },

  setupController(controller) {
    controller.set('modelWatch', this.get('watch').perform());
    return this._super(...arguments);
  },

  deactivate() {
    this.get('watch').cancelAll();
    this._super(...arguments);
  },

  watch: watchAll('node'),
});
