/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

// @ts-check
import ApplicationAdapter from './application';
import AdapterError from '@ember-data/adapter/error';
import { pluralize } from 'ember-inflector';
import classic from 'ember-classic-decorator';
import { ConflictError } from '@ember-data/adapter/error';
import DEFAULT_JOB_TEMPLATES from 'nomad-ui/utils/default-job-templates';
import { inject as service } from '@ember/service';

@classic
export default class VariableAdapter extends ApplicationAdapter {
  @service store;

  pathForType = () => 'var';

  // PUT instead of POST on create;
  // /v1/var instead of /v1/vars on create (urlForFindRecord)
  createRecord(_store, type, snapshot) {
    let data = this.serialize(snapshot);
    let baseUrl = this.buildURL(type.modelName, data.ID);
    const checkAndSetValue = snapshot?.attr('modifyIndex') || 0;
    return this.ajax(`${baseUrl}?cas=${checkAndSetValue}`, 'PUT', { data });
  }

  /**
   * Query for job templates, both defaults and variables at the nomad/job-templates path.
   * @returns {Promise<{variables: Variable[], default: Variable[]}>}
   */
  async getJobTemplates() {
    await this.populateDefaultJobTemplates();
    const jobTemplateVariables = await this.store.query('variable', {
      prefix: 'nomad/job-templates',
      namespace: '*',
    });

    // Ensure we run a findRecord on each to get its keyValues
    await Promise.all(
      jobTemplateVariables.map((t) => this.store.findRecord('variable', t.id))
    );

    const defaultTemplates = this.store
      .peekAll('variable')
      .filter((t) => t.isDefaultJobTemplate);

    return { variables: jobTemplateVariables, default: defaultTemplates };
  }

  async populateDefaultJobTemplates() {
    await Promise.all(
      DEFAULT_JOB_TEMPLATES.map((template) => {
        if (!this.store.peekRecord('variable', template.id)) {
          let variableSerializer = this.store.serializerFor('variable');
          let normalized =
            variableSerializer.normalizeDefaultJobTemplate(template);
          return this.store.createRecord('variable', normalized);
        }
        return null;
      })
    );
  }

  /**
   * @typedef Variable
   * @type {object}
   */

  /**
   * Lookup a job template variable by ID/path.
   * @param {string} templateID
   * @returns {Promise<Variable>}
   */
  async getJobTemplate(templateID) {
    await this.populateDefaultJobTemplates();
    const defaultJobs = this.store
      .peekAll('variable')
      .filter((template) => template.isDefaultJobTemplate);
    if (defaultJobs.find((job) => job.id === templateID)) {
      return defaultJobs.find((job) => job.id === templateID);
    } else {
      return this.store.findRecord('variable', templateID);
    }
  }

  urlForFindAll(modelName) {
    let baseUrl = this.buildURL(modelName);
    return pluralize(baseUrl);
  }

  urlForQuery(_query, modelName) {
    let baseUrl = this.buildURL(modelName);
    return pluralize(baseUrl);
  }

  urlForFindRecord(identifier, modelName) {
    let path,
      namespace = null;

    // TODO: Variables are namespaced. This Adapter should extend the WatchableNamespaceId Adapter.
    // When that happens, we will need to refactor this to accept JSON tuple like we do for jobs.
    const delimiter = identifier.lastIndexOf('@');
    if (delimiter !== -1) {
      path = identifier.slice(0, delimiter);
      namespace = identifier.slice(delimiter + 1);
    } else {
      path = identifier;
      namespace = 'default';
    }

    let baseUrl = this.buildURL(modelName, path);
    return `${baseUrl}?namespace=${namespace}`;
  }

  urlForUpdateRecord(identifier, modelName, snapshot) {
    const { id } = _extractIDAndNamespace(identifier, snapshot);
    let baseUrl = this.buildURL(modelName, id);
    if (snapshot?.adapterOptions?.overwrite) {
      return `${baseUrl}`;
    } else {
      const checkAndSetValue = snapshot?.attr('modifyIndex') || 0;
      return `${baseUrl}?cas=${checkAndSetValue}`;
    }
  }

  urlForDeleteRecord(identifier, modelName, snapshot) {
    const { namespace, id } = _extractIDAndNamespace(identifier, snapshot);
    const baseUrl = this.buildURL(modelName, id);
    return `${baseUrl}?namespace=${namespace}`;
  }

  handleResponse(status, _, payload) {
    if (status === 404) {
      return new AdapterError([{ detail: payload, status: 404 }]);
    }
    if (status === 409) {
      return new ConflictError([
        { detail: _normalizeConflictErrorObject(payload), status: 409 },
      ]);
    }
    return super.handleResponse(...arguments);
  }
}

function _extractIDAndNamespace(identifier, snapshot) {
  const namespace = snapshot?.attr('namespace') || 'default';
  const id = snapshot?.attr('path') || identifier;
  return {
    namespace,
    id,
  };
}

function _normalizeConflictErrorObject(conflictingVariable) {
  return {
    modifyTime: Math.floor(conflictingVariable.ModifyTime / 1000000),
    items: conflictingVariable.Items,
  };
}
