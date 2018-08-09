import { inject as service } from '@ember/service';
import { next } from '@ember/runloop';
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

  afterSetup(fn) {
    this._afterSetups || (this._afterSetups = []);
    this._afterSetups.push(fn);
  },

  beforeModel(transition) {
    return this.get('system.namespaces').then(namespaces => {
      const queryParam = transition.queryParams.namespace;
      const activeNamespace = this.get('system.activeNamespace.id');

      if (!queryParam && activeNamespace !== 'default') {
        this.afterSetup(controller => {
          controller.set('jobNamespace', activeNamespace);
        });
      } else if (queryParam && queryParam !== activeNamespace) {
        this.set('system.activeNamespace', queryParam);
      }

      return namespaces;
    });
  },

  afterModel() {
    const controller = this.controllerFor('jobs');
    next(() => {
      (this._afterSetups || []).forEach(fn => {
        fn(controller);
      });
      this._afterSetups = [];
    });
    return this._super(...arguments);
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
