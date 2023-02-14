import Route from '@ember/routing/route';

export default class DefinitionRoute extends Route {
  async model() {
    const job = this.modelFor('jobs.job');
    if (!job) return;

    const definition = await job.fetchRawDefinition();

    const definitionWithoutSpecification = { ...definition };
    delete definitionWithoutSpecification.Specification;

    const specification = await new Blob([
      definition?.Specification?.Definition,
    ]).text();

    return {
      job,
      definition: definitionWithoutSpecification,
      specification,
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
