import Route from '@ember/routing/route';
import withForbiddenState from 'nomad-ui/mixins/with-forbidden-state';
import { inject as service } from '@ember/service';

export default class VariablesRoute extends Route.extend(withForbiddenState) {
  @service can;
  @service router;

  beforeModel() {
    if (this.can.cannot('list variables')) {
      this.router.transitionTo('/jobs');
    }
  }

  model(params) {
    return {};
    //   return RSVP.hash({
    //     nodes: this.store.findAll('node'),
    //     agents: this.store.findAll('agent'),
    //   }).catch(notifyForbidden(this));
    // }
  }
}
