import Route from '@ember/routing/route';
import withForbiddenState from '../../mixins/with-forbidden-state';

export default class VariablesIndexRoute extends Route.extend(
  withForbiddenState
) {
  model() {
    // TODO: Fill in model with format from API
    return {};
    // return RSVP.hash({
    //   variables: this.store
    //     .query('variable', { namespace: params.qpNamespace })
    //     .catch(notifyForbidden(this)),
    //   namespaces: this.store.findAll('namespace'),
    // });
  }
}
