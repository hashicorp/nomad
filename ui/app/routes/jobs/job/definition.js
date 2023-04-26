/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

import Route from '@ember/routing/route';

export default class DefinitionRoute extends Route {
  async model() {
    const job = this.modelFor('jobs.job');
    if (!job) return;

    const definition = await job.fetchRawDefinition();

    const specificationResponse = await job.fetchRawSpecification();
    const specification = specificationResponse?.Source ?? null;
    const variables = specificationResponse?.VariableFlags ?? null;

    return {
      job,
      definition,
      specification,
      variables,
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

    const view = controller.view
      ? controller.view
      : model?.specification
      ? 'job-spec'
      : 'full-definition';
    controller.view = view;
  }
}
