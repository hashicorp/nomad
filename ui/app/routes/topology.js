import { inject as service } from '@ember/service';
import Route from '@ember/routing/route';
import WithForbiddenState from 'nomad-ui/mixins/with-forbidden-state';
import notifyForbidden from 'nomad-ui/utils/notify-forbidden';
import classic from 'ember-classic-decorator';
import RSVP from 'rsvp';

@classic
export default class TopologyRoute extends Route.extend(WithForbiddenState) {
  @service store;
  @service system;

  model() {
    return RSVP.hash({
      jobs: this.store.findAll('job'),
      allocations: this.store.query('allocation', {
        resources: true,
        task_states: false,
        namespace: '*',
      }),
      nodes: this.store.query('node', { resources: true }),
    }).catch(notifyForbidden(this));
  }

  setupController(controller, model) {
    // When the model throws, make sure the interface expected by the controller is consistent.
    if (!model) {
      controller.model = {
        jobs: [],
        allocations: [],
        nodes: [],
      };
    }

    return super.setupController(...arguments);
  }
}
