import Route from '@ember/routing/route';
import withForbiddenState from 'nomad-ui/mixins/with-forbidden-state';
import WithModelErrorHandling from 'nomad-ui/mixins/with-model-error-handling';
import RSVP from 'rsvp';
import notifyForbidden from 'nomad-ui/utils/notify-forbidden';
import notifyError from 'nomad-ui/utils/notify-error';

export default class VariablesVariableRoute extends Route.extend(
  withForbiddenState,
  WithModelErrorHandling
) {
  model(params) {
    // console.log('params on the way in', params, this.store.findRecord('var', params.path, { reload: true }));
    return this.store.findRecord('var', params.path, { reload: true });

    // return RSVP.hash({
    //   variables: this.store.findAll('var'),
    // }).catch(notifyForbidden(this));
  }
}
