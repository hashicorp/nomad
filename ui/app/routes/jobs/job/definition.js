import Route from '@ember/routing/route';
import EmberError from '@ember/error';

export default Route.extend({
  model() {
    const job = this.modelFor('jobs.job');

    // If there is no job, then there is no task group, so handle this as a 404
    if (!job) {
      const err = new EmberError(`Job for task group ${name} not found`);
      err.code = '404';
      this.controllerFor('application').set('error', err);
      return;
    }

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
