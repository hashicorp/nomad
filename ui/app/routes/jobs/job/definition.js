import Route from '@ember/routing/route';

export default Route.extend({
  model() {
    const job = this.modelFor('jobs.job');
    if (!job) return;

    return job.fetchRawDefinition().then(definition => ({
      job,
      definition,
    }));
  },

  resetController(controller, isExiting) {
    if (isExiting) {
      const job = controller.job;
      job.rollbackAttributes();
      job.resetId();
      controller.set('isEditing', false);
    }
  },
});
