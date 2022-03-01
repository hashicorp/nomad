import { inject as service } from '@ember/service';
import Route from '@ember/routing/route';

const ALL_NAMESPACE_WILDCARD = '*';

export default class EvaluationsIndexRoute extends Route {
  @service store;

  queryParams = {
    pageSize: {
      refreshModel: true,
    },
    nextToken: {
      refreshModel: true,
    },
    status: {
      refreshModel: true,
    },
    triggeredBy: {
      refreshModel: true,
    },
    namespace: {
      refreshModel: true,
    },
  };

  model({ pageSize, status, namespace, nextToken, triggeredBy }) {
    return this.store.query('evaluation', {
      namespace: namespace ?? ALL_NAMESPACE_WILDCARD,
      per_page: pageSize,
      next_token: nextToken,
      status,
      triggeredBy,
    });
  }
}
