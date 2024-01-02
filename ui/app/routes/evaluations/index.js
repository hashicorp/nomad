/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

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
    /*
    We use our own DSL for filter expressions. This function takes our query parameters and builds a query that matches our DSL.
    Documentation can be found here:  https://www.nomadproject.io/api-docs#filtering
    */
    const generateFilterExpression = () => {
      const searchFilter = searchTerm
        ? `ID contains "${searchTerm}" or JobID contains "${searchTerm}" or NodeID contains "${searchTerm}" or TriggeredBy contains "${searchTerm}"`
        : null;
      const typeFilter =
        type === 'client' ? `NodeID is not empty` : `NodeID is empty`;
      const triggeredByFilter = `TriggeredBy contains "${triggeredBy}"`;
      const statusFilter = `Status contains "${status}"`;

      let filterExp;
      if (searchTerm) {
        if (!type && !status && !triggeredBy) {
          return searchFilter;
        }
        filterExp = `(${searchFilter})`;
        if (type) {
          filterExp = `${filterExp} and ${typeFilter}`;
        }
        if (triggeredBy) {
          filterExp = `${filterExp} and ${triggeredByFilter}`;
        }
        if (status) {
          filterExp = `${filterExp} and ${statusFilter}`;
        }
        return filterExp;
      }

      if (type || status || triggeredBy) {
        const lookup = {
          [type]: typeFilter,
          [status]: statusFilter,
          [triggeredBy]: triggeredByFilter,
        };

        filterExp = [type, status, triggeredBy].reduce((result, filter) => {
          const expression = lookup[filter];
          if (!!filter && result !== '') {
            result = result.concat(` and ${expression}`);
          } else if (filter) {
            result = expression;
          }
          return result;
        }, '');
        return filterExp;
      }

      return null;
    };

    this.store.findAll('namespace');

    return this.store.query('evaluation', {
      namespace,
      reverse: true,
      per_page: pageSize,
      next_token: nextToken,
      filter: generateFilterExpression(),
    });
  }
}
