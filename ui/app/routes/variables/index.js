import Route from '@ember/routing/route';
import withForbiddenState from '../../mixins/with-forbidden-state';

export default class VariablesIndexRoute extends Route.extend(
  withForbiddenState
) {
  model(params) {
    return {};
    // return RSVP.hash({
    //   jobs: this.store
    //     .query('job', { namespace: params.qpNamespace })
    //     .catch(notifyForbidden(this)),
    //   namespaces: this.store.findAll('namespace'),
    // });
  }
}
