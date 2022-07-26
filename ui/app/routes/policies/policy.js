import Route from '@ember/routing/route';
import withForbiddenState from 'nomad-ui/mixins/with-forbidden-state';
import WithModelErrorHandling from 'nomad-ui/mixins/with-model-error-handling';
import { inject as service } from '@ember/service';

export default class PoliciesPolicyRoute extends Route.extend(
  withForbiddenState,
  WithModelErrorHandling
) {
  @service store;
  model() {
    // return this.store.findRecord('policy', decodeURIComponent(params.path), {
    //   reload: true,
    // });

    return {
      id: 1,
      name: 'foo',
      description: 'bar',
      rules: `
        foo = "bar"
      `,
    };
  }
}
