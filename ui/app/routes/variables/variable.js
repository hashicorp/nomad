import Route from '@ember/routing/route';
import withForbiddenState from 'nomad-ui/mixins/with-forbidden-state';
import WithModelErrorHandling from 'nomad-ui/mixins/with-model-error-handling';

export default class VariablesVariableRoute extends Route.extend(
  withForbiddenState,
  WithModelErrorHandling
) {
  model(params) {
    return this.store.findRecord('var', params.path, { reload: true });
  }
}
