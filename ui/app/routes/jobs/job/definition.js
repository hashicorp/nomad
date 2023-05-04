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

    let format = 'json'; // default to json in network request errors
    let specification;
    let variableFlags;
    let variableLiteral;
    try {
      const specificationResponse = await job.fetchRawSpecification();
      specification = specificationResponse?.Source ?? null;
      variableFlags = specificationResponse?.VariableFlags ?? null;
      variableLiteral = specificationResponse?.Variables ?? null;
      format = specificationResponse?.Format ?? 'json';
    } catch (e) {
      // Swallow the error because Nomad job pre-1.6 will not have a specification
    }

    return {
      definition,
      format,
      job,
      specification,
      variableFlags,
      variableLiteral,
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
