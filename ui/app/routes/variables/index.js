import Route from '@ember/routing/route';
import withForbiddenState from '../../mixins/with-forbidden-state';
import RSVP from 'rsvp';
import notifyForbidden from 'nomad-ui/utils/notify-forbidden';

export default class VariablesIndexRoute extends Route.extend(
  withForbiddenState
) {
  model(params) {
    // return {};
    return RSVP.hash({
      variables: this.store
        .query('var', { namespace: params.qpNamespace })
        .catch(notifyForbidden(this)),
      namespaces: this.store.findAll('namespace'),
    });
  }
}
