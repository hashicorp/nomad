import { inject as service } from '@ember/service';
import Controller from '@ember/controller';
import { observer } from '@ember/object';
import { run } from '@ember/runloop';

export default Controller.extend({
  system: service(),

  queryParams: {
    jobNamespace: 'namespace',
  },

  isForbidden: false,

  jobNamespace: 'default',

  // The namespace query param should act as an alias to the system active namespace.
  // But query param defaults can't be CPs: https://github.com/emberjs/ember.js/issues/9819
  syncNamespaceService: forwardNamespace('jobNamespace', 'system.activeNamespace'),
  syncNamespaceParam: forwardNamespace('system.activeNamespace', 'jobNamespace'),

  actions: {
    refreshRoute() {
      return true;
    },
  },
});

function forwardNamespace(source, destination) {
  return observer(source, `${source}.id`, function() {
    const newNamespace = this.get(`${source}.id`) || this.get(source);
    const currentNamespace = this.get(`${destination}.id`) || this.get(destination);
    const bothAreDefault =
      (currentNamespace == undefined || currentNamespace === 'default') &&
      (newNamespace == undefined || newNamespace === 'default');

    if (currentNamespace !== newNamespace && !bothAreDefault) {
      this.set(destination, newNamespace);
      run.next(() => {
        this.send('refreshRoute');
      });
    }
  });
}
