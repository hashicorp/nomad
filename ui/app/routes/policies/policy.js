import Route from '@ember/routing/route';
import withForbiddenState from 'nomad-ui/mixins/with-forbidden-state';
import WithModelErrorHandling from 'nomad-ui/mixins/with-model-error-handling';
import { inject as service } from '@ember/service';

export default class PoliciesPolicyRoute extends Route.extend(
  withForbiddenState,
  WithModelErrorHandling
) {
  @service store;
  model(params) {
    return this.store.findRecord('policy', decodeURIComponent(params.name), {
      reload: true,
    });
  }
}
