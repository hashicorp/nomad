import Ember from 'ember';

const { Controller, inject, observer } = Ember;

export default Controller.extend({
  system: inject.service(),

  queryParams: {
    jobNamespace: 'namespace',
  },

  jobNamespace: 'default',

  // The namespace query param should act as an alias to the system active namespace.
  // But query param defaults can't be CPs: https://github.com/emberjs/ember.js/issues/9819
  syncNamespaceService: observer('jobNamespace', function() {
    const newNamespace = this.get('jobNamespace');
    const currentNamespace = this.get('system.activeNamespace.id');
    const bothAreDefault =
      (currentNamespace == undefined || currentNamespace === 'default') &&
      (newNamespace == undefined || newNamespace === 'default');

    if (currentNamespace !== newNamespace && !bothAreDefault) {
      this.set('system.activeNamespace', newNamespace);
      this.send('refreshRoute');
    }
  }),
});
