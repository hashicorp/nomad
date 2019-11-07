import Controller from '@ember/controller';

export default Controller.extend({
  onSubmit(id, namespace) {
    this.transitionToRoute('jobs.job', id, {
      queryParams: { jobNamespace: namespace },
    });
  },
});
