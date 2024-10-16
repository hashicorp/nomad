/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

// @ts-check
import Route from '@ember/routing/route';
import { inject as service } from '@ember/service';
/**
 * Route for fetching and displaying a job's definition and specification.
 */
export default class DefinitionRoute extends Route {
  @service notifications;

  queryParams = {
    version: {
      refreshModel: true,
    },
  };

  /**
   * Fetch the job's definition, specification, and variables from the API.
   *
   * @returns {Promise<Object>} A promise that resolves to an object containing the job, definition, format,
   *                            specification, variableFlags, and variableLiteral.
   */
  async model({ version }) {
    version = +version; // query parameter is a string; convert to number
    /** @type {import('../../../models/job').default} */
    const job = this.modelFor('jobs.job');
    if (!job) return;

    let definition;

    if (version) {
      try {
        const versionResponse = await job.getVersions();
        const versions = versionResponse.Versions;
        definition = versions.findBy('Version', version);
        if (!definition) {
          throw new Error('Version not found');
        }
      } catch (e) {
        console.error('error fetching job version definition', e);
        this.notifications.add({
          title: 'Error Fetching Job Version Definition',
          message: `There was an error fetching the versions for this job: ${e.message}`,
          color: 'critical',
        });
      }
    } else {
      definition = await job.fetchRawDefinition();
    }

    let format = 'json'; // default to json in network request errors
    let specification;
    let variableFlags;
    let variableLiteral;
    try {
      const specificationResponse = await job.fetchRawSpecification(version);
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

  /**
   * Reset the controller when exiting the route.
   *
   * @param {Controller} controller - The controller instance.
   * @param {boolean} isExiting - A boolean flag indicating if the route is being exited.
   */
  resetController(controller, isExiting) {
    if (isExiting) {
      const job = controller.job;
      job.rollbackAttributes();
      job.resetId();
      controller.set('isEditing', false);
    }
  }

  /**
   * Set up the controller with the model data and determine the view type.
   *
   * @param {Controller} controller - The controller instance.
   * @param {Object} model - The model data fetched in the `model` method.
   */
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
