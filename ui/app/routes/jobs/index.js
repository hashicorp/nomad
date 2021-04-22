import { inject as service } from '@ember/service';
import Route from '@ember/routing/route';
import RSVP from 'rsvp';
import { collect } from '@ember/object/computed';
import { watchAll, watchQuery } from 'nomad-ui/utils/properties/watch';
import WithWatchers from 'nomad-ui/mixins/with-watchers';
import notifyForbidden from 'nomad-ui/utils/notify-forbidden';
import WithForbiddenState from 'nomad-ui/mixins/with-forbidden-state';

export default class IndexRoute extends Route.extend(WithWatchers, WithForbiddenState) {
  @service store;

  queryParams = {
    qpNamespace: {
      refreshModel: true,
    },
  };

  model(params) {
    return RSVP.hash({
      jobs: this.store.query('job', { namespace: params.qpNamespace }),
      namespaces: this.store.findAll('namespace'),
    }).catch(notifyForbidden(this));
  }

  startWatchers(controller) {
    controller.set('namespacesWatch', this.watchNamespaces.perform());
    controller.set('modelWatch', this.watchJobs.perform());
  }

  @watchQuery('job') watchJobs;
  @watchAll('namespace') watchNamespaces;
  @collect('watchJobs', 'watchNamespaces') watchers;
}
