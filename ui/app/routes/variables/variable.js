import Route from '@ember/routing/route';
import withForbiddenState from 'nomad-ui/mixins/with-forbidden-state';
import WithModelErrorHandling from 'nomad-ui/mixins/with-model-error-handling';
import { inject as service } from '@ember/service';

export default class VariablesVariableRoute extends Route.extend(
  withForbiddenState,
  WithModelErrorHandling
) {
  @service store;
  model(params) {
    return this.store.findRecord('variable', decodeURIComponent(params.path), {
      reload: true,
    });
  }
}
