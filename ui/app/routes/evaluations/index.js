import { inject as service } from '@ember/service';
import Route from '@ember/routing/route';

const ALL_NAMESPACE_WILDCARD = '*';

export default class EvaluationsIndexRoute extends Route {
  @service store;

  queryParams = {
    pageSize: {
      refreshModel: true,
    },
  };

  model({ pageSize }) {
    return this.store.query('evaluation', {
      namespace: ALL_NAMESPACE_WILDCARD,
      per_page: pageSize,
    });
  }
}
