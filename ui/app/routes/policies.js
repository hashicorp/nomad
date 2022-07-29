import Route from '@ember/routing/route';
import withForbiddenState from 'nomad-ui/mixins/with-forbidden-state';
import WithModelErrorHandling from 'nomad-ui/mixins/with-model-error-handling';
import { inject as service } from '@ember/service';

export default class PoliciesRoute extends Route.extend(
  withForbiddenState,
  WithModelErrorHandling
) {
  @service can;
  @service store;

  beforeModel() {
    if (this.can.cannot('list policies')) {
      this.router.transitionTo('/jobs');
    }
  }

  async model(params, b, c) {
    console.log({ params }, b, c);
    const policies = await this.store.query('policy', { reload: true });
    return policies;
  }
}
