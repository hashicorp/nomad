import Route from '@ember/routing/route';
import { inject as service } from '@ember/service';
import WithForbiddenState from 'nomad-ui/mixins/with-forbidden-state';
import RSVP from 'rsvp';
import notifyForbidden from 'nomad-ui/utils/notify-forbidden';

export default class VariablesRoute extends Route.extend(WithForbiddenState) {
  @service can;
  @service router;

  beforeModel() {
    if (this.can.cannot('list variables')) {
      this.router.transitionTo('/jobs');
    }
  }
  model() {
    return RSVP.hash({
      variables: this.store.findAll('variable'),
    }).catch(notifyForbidden(this));
  }
}
