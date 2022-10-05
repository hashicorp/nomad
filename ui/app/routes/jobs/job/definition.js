import Route from '@ember/routing/route';

export default class DefinitionRoute extends Route {
  async model() {
    const job = this.modelFor('jobs.job');
    if (!job) return;

    const definition = await job.fetchRawDefinition();

    return {
      job,
      definition,
    };
  }

  resetController(controller, isExiting) {
    if (isExiting) {
      const job = controller.job;
      job.rollbackAttributes();
      job.resetId();
      controller.set('isEditing', false);
    }
  }
}
