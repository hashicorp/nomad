import { inject as service } from '@ember/service';
import Route from '@ember/routing/route';

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
    qpNamespace: {
      refreshModel: true,
    },
  };

  model({ pageSize, status, qpNamespace: namespace, nextToken, triggeredBy }) {
    this.store.findAll('namespace');
    return this.store.query('evaluation', {
      namespace,
      per_page: pageSize,
      next_token: nextToken,
      status,
      triggeredBy,
    });
  }
}
