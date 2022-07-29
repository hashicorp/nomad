import Route from '@ember/routing/route';
import { inject as service } from '@ember/service';

export default class PoliciesIndexRoute extends Route {
  @service can;
  @service router;

  beforeModel() {
    if (this.can.cannot('list policies')) {
      this.router.transitionTo('/jobs');
    }
  }
}
