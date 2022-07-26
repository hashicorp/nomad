import Route from '@ember/routing/route';
import withForbiddenState from 'nomad-ui/mixins/with-forbidden-state';
import WithModelErrorHandling from 'nomad-ui/mixins/with-model-error-handling';
import { inject as service } from '@ember/service';

export default class PoliciesRoute extends Route.extend(withForbiddenState) {
  @service store;
  async model(params) {
    console.log({ params });
    try {
      const policies = await this.store.query('policy', { reload: true });
      console.log('and thus', { policies });

      return policies;
    } catch (e) {
      notifyError(this)(e);
    }
  }
}
