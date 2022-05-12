import Route from '@ember/routing/route';
import withForbiddenState from 'nomad-ui/mixins/with-forbidden-state';

export default class VariablesRoute extends Route.extend(withForbiddenState) {
  model(params) {
    return {};
    //   return RSVP.hash({
    //     nodes: this.store.findAll('node'),
    //     agents: this.store.findAll('agent'),
    //   }).catch(notifyForbidden(this));
    // }
  }
}
