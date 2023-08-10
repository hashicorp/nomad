/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

// @ts-check
import Component from '@glimmer/component';
import { inject as service } from '@ember/service';
import { action } from '@ember/object';
import { task } from 'ember-concurrency';
import messageFromAdapterError from 'nomad-ui/utils/message-from-adapter-error';
import localStorageProperty from 'nomad-ui/utils/properties/local-storage';
import { tracked } from '@glimmer/tracking';

/**
 * JobEditor component that provides an interface for editing and managing Nomad jobs.
 *
 * @class JobEditor
 * @extends Component
 */
export default class JobEditor extends Component {
  @service config;
  @service store;
  @service notifications;

  @tracked error = null;
  @tracked planOutput = null;

  /**
   * Initialize the component, setting the definition and definition variables on the model if available.
   */
  constructor() {
    super(...arguments);

    if (this.definition) {
      this.setDefinitionOnModel();
    }

    if (this.args.variables) {
      this.args.job.set(
        '_newDefinitionVariables',
        this.jsonToHcl(this.args.variables.flags).concat(
          this.args.variables.literal
        )
      );
    }
  }

  /**
   * Check if the component is in editing mode.
   *
   * @returns {boolean} True if the component is in 'new' or 'edit' context, otherwise false.
   */
  get isEditing() {
    return ['new', 'edit'].includes(this.args.context);
  }

  @action
  setDefinitionOnModel() {
    this.args.job.set('_newDefinition', this.definition);
  }

  /**
   * Enter the edit mode and defensively set the definition on the model.
   */
  @action
  edit() {
    this.setDefinitionOnModel();
    this.args.onToggleEdit(true);
  }

  @action
  onCancel() {
    this.args.onToggleEdit(false);
  }

  /**
   * Determine the current stage of the component based on the plan output and editing state.
   *
   * @returns {"review"|"edit"|"read"} The current stage, either 'review', 'edit', or 'read'.
   */
  get stage() {
    if (this.planOutput) return 'review';
    if (this.isEditing) return 'edit';
    else return 'read';
  }

  @localStorageProperty('nomadMessageJobPlan', true) shouldShowPlanMessage;
  @localStorageProperty('nomadShouldWrapCode', false) shouldWrapCode;

  @action
  dismissPlanMessage() {
    this.shouldShowPlanMessage = false;
  }

  /**
   * A task that performs the job parsing and planning.
   * On error, it calls the onError method.
   */
  @(task(function* () {
    this.reset();

    try {
      yield this.args.job.parse();
    } catch (err) {
      this.onError(err, 'parse', 'parse jobs');
      return;
    }

    try {
      const plan = yield this.args.job.plan();
      this.planOutput = plan;
    } catch (err) {
      this.onError(err, 'plan', 'plan jobs');
    }
  }).drop())
  plan;

  /**
   * A task that submits the job, either running a new job or updating an existing one.
   * On error, it calls the onError method and resets our planOutput state.
   */
  @task(function* () {
    try {
      if (this.args.context === 'new') {
        yield this.args.job.run();
      } else {
        yield this.args.job.update(this.args.format);
      }

      const id = this.args.job.plainId;
      const namespace = this.args.job.belongsTo('namespace').id() || 'default';

      this.reset();

      // Treat the job as ephemeral and only provide ID parts.
      this.args.onSubmit(id, namespace);
    } catch (err) {
      this.onError(err, 'run', 'submit jobs');
      this.planOutput = null;
    }
  })
  submit;

  /**
   * Handle errors, setting the error object and scrolling to the error message.
   *
   * @param {Error} err - The error object.
   * @param {"parse"|"plan"|"run"} type - The type of error (e.g., 'parse', 'plan', 'run').
   * @param {string} actionMsg - A message describing the action that caused the error.
   */
  onError(err, type, actionMsg) {
    const error = messageFromAdapterError(err, actionMsg);
    this.error = { message: error, type };
    this.scrollToError();
  }

  @action
  reset() {
    this.planOutput = null;
    this.error = null;
  }

  scrollToError() {
    if (!this.config.get('isTest')) {
      window.scrollTo(0, 0);
    }
  }

  /**
   * Update the job's definition or definition variables based on the provided type.
   *
   * @param {string} value - The new value for the job's definition or definition variables.
   * @param {_codemirror} _codemirror - The CodeMirror instance (not used in this action).
   * @param {"hclVariables"|"job"} [type='job'] - The type of code being updated ('job' or 'hclVariables').
   */
  @action
  updateCode(value, _codemirror, type = 'job') {
    if (!this.args.job.isDestroying && !this.args.job.isDestroyed) {
      if (type === 'hclVariables') {
        this.args.job.set('_newDefinitionVariables', value);
      } else {
        this.args.job.set('_newDefinition', value);
      }
    }
  }

  /**
   * Toggle the wrapping of the job's definition or definition variables.
   */
  @action
  toggleWrap() {
    this.shouldWrapCode = !this.shouldWrapCode;
  }

  /**
   * Read the content of an uploaded job specification file and update the job's definition.
   *
   * @param {Event} event - The input change event containing the selected file.
   */
  @action
  uploadJobSpec(event) {
    const reader = new FileReader();
    reader.onload = () => {
      this.updateCode(reader.result);
    };

    const [file] = event.target.files;
    reader.readAsText(file);
  }

  /**
   * Download the job's definition or specification as .nomad.hcl file locally
   */
  @action
  async handleSaveAsFile() {
    try {
      const blob = new Blob([this.args.job._newDefinition], {
        type: 'text/plain',
      });
      const url = window.URL.createObjectURL(blob);
      const downloadAnchor = document.createElement('a');

      downloadAnchor.href = url;
      downloadAnchor.target = '_blank';
      downloadAnchor.rel = 'noopener noreferrer';
      downloadAnchor.download = 'jobspec.nomad.hcl';

      downloadAnchor.click();
      downloadAnchor.remove();

      window.URL.revokeObjectURL(url);
      this.notifications.add({
        title: 'jobspec.nomad.hcl has been downloaded',
        color: 'success',
        icon: 'download',
      });
    } catch (err) {
      this.notifications.add({
        title: 'Error downloading file',
        message: err.message,
        color: 'critical',
        sticky: true,
      });
    }
  }

  /**
   * Get the definition or specification based on the view type.
   *
   * @returns {string} The definition or specification in JSON or HCL format.
   */
  get definition() {
    if (this.args.view === 'full-definition') {
      return JSON.stringify(this.args.definition, null, 2);
    } else {
      return this.args.specification;
    }
  }

  /**
   * Convert a JSON object to an HCL string.
   *
   * @param {Object} obj - The JSON object to convert.
   * @returns {string} The HCL string representation of the JSON object.
   */
  jsonToHcl(obj) {
    const hclLines = [];

    for (const key in obj) {
      const value = obj[key];
      const hclValue = typeof value === 'string' ? `"${value}"` : value;
      hclLines.push(`${key}=${hclValue}\n`);
    }

    return hclLines.join('\n');
  }

  get data() {
    return {
      cancelable: this.args.cancelable,
      definition: this.definition,
      format: this.args.format,
      hasSpecification: !!this.args.specification,
      hasVariables:
        !!this.args.variables?.flags || !!this.args.variables?.literal,
      job: this.args.job,
      planOutput: this.planOutput,
      shouldShowPlanMessage: this.shouldShowPlanMessage,
      view: this.args.view,
      shouldWrap: this.shouldWrapCode,
    };
  }

  get fns() {
    return {
      onCancel: this.onCancel,
      onDismissPlanMessage: this.dismissPlanMessage,
      onEdit: this.edit,
      onPlan: this.plan,
      onReset: this.reset,
      onSaveAs: this.args.handleSaveAsTemplate,
      onSaveFile: this.handleSaveAsFile,
      onSubmit: this.submit,
      onSelect: this.args.onSelect,
      onUpdate: this.updateCode,
      onUpload: this.uploadJobSpec,
      onToggleWrap: this.toggleWrap,
    };
  }
}
