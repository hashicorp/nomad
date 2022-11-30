import Route from '@ember/routing/route';
import withForbiddenState from 'nomad-ui/mixins/with-forbidden-state';
import WithModelErrorHandling from 'nomad-ui/mixins/with-model-error-handling';
import { inject as service } from '@ember/service';

export default class PoliciesPolicyRoute extends Route.extend(
  withForbiddenState,
  WithModelErrorHandling
) {
  @service store;
  async model(params) {
    const policy = await this.store.findRecord('policy', decodeURIComponent(params.name), {
      reload: true,
    });
    const tokens = this.store.peekAll('token').filter(token => token.policyNames?.includes(policy.name));
    return { policy, tokens };
  }
}
