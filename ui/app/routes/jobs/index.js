import { inject as service } from '@ember/service';
import Route from '@ember/routing/route';
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
  };

  async model(params) {
    const [jobs, namespaces] = await Promise.all([
      this.store.query('job', { namespace: params.qpNamespace }),
      this.store.findAll('namespace'),
    ]);

    try {
      return {
        jobs,
        namespaces,
      };
    } catch (e) {
      notifyForbidden(this)(e);
    }
  }

  startWatchers(controller) {
    controller.set('namespacesWatch', this.watchNamespaces.perform());
    controller.set(
      'modelWatch',
      this.watchJobs.perform({ namespace: controller.qpNamesapce })
    );
  }

  @watchQuery('job') watchJobs;
  @watchAll('namespace') watchNamespaces;
  @collect('watchJobs', 'watchNamespaces') watchers;
}
