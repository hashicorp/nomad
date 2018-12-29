import { inject as service } from '@ember/service';
import Route from '@ember/routing/route';
import WithForbiddenState from 'nomad-ui/mixins/with-forbidden-state';
import notifyForbidden from 'nomad-ui/utils/notify-forbidden';

export default Route.extend(WithForbiddenState, {
  system: service(),
  store: service(),

  breadcrumbs: [
    {
      label: 'Jobs',
      args: ['jobs.index'],
    },
  ],

  queryParams: {
    jobNamespace: {
      refreshModel: true,
    },
  },

  beforeModel(transition) {
    return this.get('system.namespaces').then(namespaces => {
      const queryParam = transition.queryParams.namespace;
      this.set('system.activeNamespace', queryParam || 'default');

      return namespaces;
    });
  },

  model() {
    return this.get('store')
      .findAll('job', { reload: true })
      .catch(notifyForbidden(this));
  },

  actions: {
    refreshRoute() {
      this.refresh();
    },
  },
});
