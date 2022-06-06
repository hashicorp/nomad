import Route from '@ember/routing/route';
import { inject as service } from '@ember/service';
import WithForbiddenState from 'nomad-ui/mixins/with-forbidden-state';
import notifyError from 'nomad-ui/utils/notify-error';
import PathTree from 'nomad-ui/utils/path-tree';

export default class VariablesRoute extends Route.extend(WithForbiddenState) {
  @service can;
  @service router;
  @service store;

  beforeModel() {
    if (this.can.cannot('list variables')) {
      this.router.transitionTo('/jobs');
    }
  }
  async model() {
    try {
      const variables = await this.store.findAll('variable');
      return {
        variables,
        pathTree: new PathTree(variables),
      };
    } catch (e) {
      notifyError(this)(e);
    }
  }
}
