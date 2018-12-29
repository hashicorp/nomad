import Route from '@ember/routing/route';

export default Route.extend({
  model() {
    const job = this.modelFor('jobs.job');
    return job.fetchRawDefinition().then(definition => ({
      job,
      definition,
    }));
  },

  resetController(controller, isExiting) {
    if (isExiting) {
      const job = controller.get('job');
      job.rollbackAttributes();
      job.resetId();
      controller.set('isEditing', false);
    }
  },
});
