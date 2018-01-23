import { inject as service } from '@ember/service';
import Route from '@ember/routing/route';
import { run } from '@ember/runloop';
import WithForbiddenState from 'nomad-ui/mixins/with-forbidden-state';
import notifyForbidden from 'nomad-ui/utils/notify-forbidden';

export default Route.extend(WithForbiddenState, {
  system: service(),
  store: service(),

  beforeModel() {
    return this.get('system.namespaces');
  },

  model() {
    return this.get('store')
      .findAll('job')
      .catch(notifyForbidden(this));
  },

  syncToController(controller) {
    const namespace = this.get('system.activeNamespace.id');

    // The run next is necessary to let the controller figure
    // itself out before updating QPs.
    // See: https://github.com/emberjs/ember.js/issues/5465
    run.next(() => {
      if (namespace && namespace !== 'default') {
        controller.set('jobNamespace', namespace);
      } else {
        controller.set('jobNamespace', 'default');
      }
    });
  },

  setupController(controller) {
    this.syncToController(controller);
    return this._super(...arguments);
  },

  actions: {
    refreshRoute() {
      this.refresh();
    },
  },
});
