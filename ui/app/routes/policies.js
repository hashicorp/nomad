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
  @service router;

  beforeModel() {
    if (this.can.cannot('list policies')) {
      this.router.transitionTo('/jobs');
    }
  }

  async model() {
    const policies = await this.store.query('policy', { reload: true });
    const tokens = await this.store.query('token', { reload: true });
    return { policies, tokens };
  }
}
