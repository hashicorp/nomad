/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

import Component from '@glimmer/component';
import { inject as service } from '@ember/service';
import { action } from '@ember/object';
import { task } from 'ember-concurrency';
import messageFromAdapterError from 'nomad-ui/utils/message-from-adapter-error';
import localStorageProperty from 'nomad-ui/utils/properties/local-storage';
import { tracked } from '@glimmer/tracking';

export default class JobEditor extends Component {
  @service config;
  @service store;

  @tracked error = null;
  @tracked planOutput = null;

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

  get isEditing() {
    return ['new', 'edit'].includes(this.args.context);
  }

  @action
  setDefinitionOnModel() {
    this.args.job.set('_newDefinition', this.definition);
  }

  @action
  edit() {
    this.setDefinitionOnModel();
    this.args.onToggleEdit(true);
  }

  @action
  onCancel() {
    this.args.onToggleEdit(false);
  }

  get stage() {
    if (this.planOutput) return 'review';
    if (this.isEditing) return 'edit';
    else return 'read';
  }

  @localStorageProperty('nomadMessageJobPlan', true) shouldShowPlanMessage;

  @action
  dismissPlanMessage() {
    this.shouldShowPlanMessage = false;
  }

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

  @action
  uploadJobSpec(event) {
    const reader = new FileReader();
    reader.onload = () => {
      this.updateCode(reader.result);
    };

    const [file] = event.target.files;
    reader.readAsText(file);
  }

  get definition() {
    if (this.args.view === 'full-definition') {
      return JSON.stringify(this.args.definition, null, 2);
    } else {
      return this.args.specification;
    }
  }

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
      onSubmit: this.submit,
      onSelect: this.args.onSelect,
      onUpdate: this.updateCode,
      onUpload: this.uploadJobSpec,
    };
  }
}
