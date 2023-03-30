import Route from '@ember/routing/route';

export default class DefinitionRoute extends Route {
  async model() {
    const job = this.modelFor('jobs.job');
    if (!job) return;

    const definition = await job.fetchRawDefinition();

    const hasSpecification = !!definition?.Specification;

    const specification = hasSpecification
      ? await new Blob([definition?.Specification?.Definition]).text()
      : null;

    return {
      job,
      definition,
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

  setupController(controller, model) {
    super.setupController(controller, model);
    controller.view =
      model?.specification && !controller.view ? 'job-spec' : 'full-definition';
  }
}
