import { inject as service } from '@ember/service';
import Route from '@ember/routing/route';
import WithForbiddenState from 'nomad-ui/mixins/with-forbidden-state';
import notifyForbidden from 'nomad-ui/utils/notify-forbidden';
import classic from 'ember-classic-decorator';

@classic
export default class TopologyRoute extends Route.extend(WithForbiddenState) {
  @service store;
  @service system;

  async model() {
    try {
      const [jobs, allocations, nodes] = await Promise.all([
        this.store.findAll('job'),
        this.store.query('allocation', {
          resources: true,
          task_states: false,
          namespace: '*',
        }),
        this.store.query('node', { resources: true }),
      ]);

      return {
        jobs,
        allocations,
        nodes,
      };
    } catch (e) {
      notifyForbidden(this)(e);
    }
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
