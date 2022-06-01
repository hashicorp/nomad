import Route from '@ember/routing/route';
import { inject as service } from '@ember/service';
import WithForbiddenState from 'nomad-ui/mixins/with-forbidden-state';
import RSVP from 'rsvp';
import notifyForbidden from 'nomad-ui/utils/notify-forbidden';
import PathTree from 'nomad-ui/utils/path-tree';

export default class VariablesRoute extends Route.extend(WithForbiddenState) {
  @service can;
  @service router;

  beforeModel() {
    if (this.can.cannot('list variables')) {
      this.router.transitionTo('/jobs');
    }
  }
  async model() {
    const variables = await this.store.findAll('variable');
    return RSVP.hash({
      variables,
      pathTree: new PathTree(variables),
    }).catch(notifyForbidden(this));
  }
}
