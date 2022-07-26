import Route from '@ember/routing/route';
import withForbiddenState from 'nomad-ui/mixins/with-forbidden-state';
import WithModelErrorHandling from 'nomad-ui/mixins/with-model-error-handling';
import { inject as service } from '@ember/service';

export default class PoliciesRoute extends Route.extend(withForbiddenState) {
  @service store;
  async model(params, b, c) {
    console.log({ params }, b, c);
    try {
      const policies = await this.store.query('policy', { reload: true });
      return policies;
    } catch (e) {
      notifyError(this)(e);
    }
  }
}
