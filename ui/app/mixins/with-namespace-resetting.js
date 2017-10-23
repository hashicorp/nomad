import Ember from 'ember';

const { Mixin, inject } = Ember;

export default Mixin.create({
  system: inject.service(),
  jobsController: inject.controller('jobs'),

  actions: {
    gotoJobs(namespace) {
      // Since the setupController hook doesn't fire when transitioning up the
      // route hierarchy, the two sides of the namespace bindings need to be manipulated
      // in order for the jobs route model to reload.
      this.set('system.activeNamespace', this.get('jobsController.jobNamespace'));
      this.set('jobsController.jobNamespace', namespace.get('id'));
      this.transitionToRoute('jobs');
    },
  },
});
