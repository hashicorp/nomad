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

    const hasSpecification = !!definition?.Specification;
    const hasHCLVariables = !!definition?.Specification?.Variables;

    const specification = hasSpecification
      ? await new Blob([definition?.Specification?.Definition]).text()
      : null;

    const variables = hasHCLVariables
      ? definition?.Specification?.Variables
      : null;

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
}
