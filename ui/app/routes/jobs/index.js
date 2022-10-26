import { inject as service } from '@ember/service';
import Route from '@ember/routing/route';
import RSVP from 'rsvp';
import { collect } from '@ember/object/computed';
import { watchAll, watchQuery } from 'nomad-ui/utils/properties/watch';
import WithWatchers from 'nomad-ui/mixins/with-watchers';
import notifyForbidden from 'nomad-ui/utils/notify-forbidden';
import WithForbiddenState from 'nomad-ui/mixins/with-forbidden-state';

export default class IndexRoute extends Route.extend(
  WithWatchers,
  WithForbiddenState
) {
  @service store;

  queryParams = {
    qpNamespace: {
      refreshModel: true,
    },
    pageSize: {
      refreshModel: true,
    },
    nextToken: {
      refreshModel: true,
    },
    status: {
      refreshModel: true,
    },
    type: {
      refreshModel: true,
    },
    searchTerm: {
      refreshModel: true,
    },
    datacenter: {
      refreshModel: true,
    },
    prefix: {
      refreshModel: true,
    },
  };

  model({
    searchTerm,
    type,
    qpNamespace,
    pageSize,
    status,
    nextToken,
    datacenter,
  }) {
    const parseJSON = (qp) => (qp ? JSON.parse(qp) : null);
    const iterateOverList = (list, type) => {
      const dictionary = {
        type: ['Type', '=='],
        status: ['Status', '=='],
        datacenter: ['Datacenters', 'contains'],
      };
      const [selector, matcher] = dictionary[type];
      if (!list) return null;
      return list.reduce((accum, val, idx) => {
        if (idx !== list.length - 1) {
          accum += `${selector} ${matcher} "${val}" or `;
        } else {
          accum += `${selector} ${matcher} "${val}")`;
        }
        return accum;
      }, '(');
    };
    /*
    We use our own DSL for filter expressions. This function takes our query parameters and builds a query that matches our DSL.
    Documentation can be found here:  https://www.nomadproject.io/api-docs#filtering
    */
    const generateFilterExpression = () => {
      const searchFilter = searchTerm ? `Name contains "${searchTerm}"` : null;
      const typeFilter = iterateOverList(parseJSON(type), 'type');
      const datacenterFilter = iterateOverList(
        parseJSON(datacenter),
        'datacenter'
      );
      const statusFilter = iterateOverList(parseJSON(status), 'status');

      let filterExp;
      if (searchTerm) {
        if (!type && !status && !datacenter) {
          return searchFilter;
        }
        filterExp = `(${searchFilter})`;
        if (type) {
          filterExp = `${filterExp} and ${typeFilter}`;
        }
        if (datacenter) {
          filterExp = `${filterExp} and ${datacenterFilter}`;
        }
        if (status) {
          filterExp = `${filterExp} and ${statusFilter}`;
        }
        return filterExp;
      }

      if (type || status || datacenter) {
        const lookup = {
          [type]: typeFilter,
          [status]: statusFilter,
          [datacenter]: datacenterFilter,
        };

        filterExp = [type, status, datacenter].reduce((result, filter) => {
          const expression = lookup[filter];
          if (!!filter && result !== '') {
            result = result.concat(` and ${expression}`);
          } else if (filter) {
            result = expression;
          }
          return result;
        }, '');
        debugger;
        return filterExp;
      }

      return null;
    };

    const hasFilters = !!generateFilterExpression();

    return RSVP.hash({
      jobs: this.store
        .query('job', {
          namespace: qpNamespace,
          per_page: pageSize,
          filter: hasFilters
            ? `ParentID is empty and ${generateFilterExpression()}`
            : `ParentID is empty`,
          next_token: nextToken,
        })
        .catch(notifyForbidden(this)),
      namespaces: this.store.findAll('namespace'),
    });
  }

  startWatchers(controller) {
    controller.set('namespacesWatch', this.watchNamespaces.perform());
    controller.set(
      'modelWatch',
      this.watchJobs.perform({
        namespace: controller.qpNamespace,
        per_page: controller.pageSize,
        filter: `ParentID is empty`,
        next_token: controller.nextToken,
      })
    );
  }

  @watchQuery('job') watchJobs;
  @watchAll('namespace') watchNamespaces;
  @collect('watchJobs', 'watchNamespaces') watchers;
}
