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
    searchTerm: {
      refreshModel: true,
    },
    type: {
      refreshModel: true,
    },
  };

  model({
    nextToken,
    pageSize,
    searchTerm,
    status,
    triggeredBy,
    type,
    qpNamespace: namespace,
  }) {
    const generateFilter = () => {
      const searchFilter = searchTerm
        ? `ID contains "${searchTerm}" or JobID contains "${searchTerm}" or NodeID contains "${searchTerm}" or TriggeredBy contains "${searchTerm}"`
        : null;
      const typeFilter =
        type === 'client' ? `NodeID is not empty` : `NodeID is empty`;

      if (searchTerm && type) return `${searchFilter} ${typeFilter}`;
      if (searchTerm) return searchFilter;
      if (type) return typeFilter;

      return null;
    };

    this.store.findAll('namespace');

    return this.store.query('evaluation', {
      namespace,
      per_page: pageSize,
      next_token: nextToken,
      status,
      triggeredBy,
      filter: generateFilter(),
    });
  }
}
