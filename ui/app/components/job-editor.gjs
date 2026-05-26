/**
 * Copyright IBM Corp. 2015, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';
import { tracked } from '@glimmer/tracking';
import { on } from '@ember/modifier';
import { service } from '@ember/service';
import { scheduleOnce } from '@ember/runloop';
import { task } from 'ember-concurrency';
import can from 'ember-can/helpers/can';
import { eq } from 'ember-truth-helpers';
import {
  HdsButton,
  HdsButtonSet,
} from '@hashicorp/design-system-components/components';
import JobEditorAlert from 'nomad-ui/components/job-editor/alert';
import JobEditorEdit from 'nomad-ui/components/job-editor/edit';
import JobEditorRead from 'nomad-ui/components/job-editor/read';
import JobEditorReview from 'nomad-ui/components/job-editor/review';
import messageFromAdapterError from 'nomad-ui/utils/message-from-adapter-error';
import jsonToHcl from 'nomad-ui/utils/json-to-hcl';
import localStorageProperty from 'nomad-ui/utils/properties/local-storage';

export default class JobEditor extends Component {
  @service config;
  @service store;
  @service notifications;

  @tracked error = null;
  @tracked planOutput = null;

  constructor() {
    super(...arguments);

    const isEmberDataModel = typeof this.args.job?.belongsTo === 'function';
    const shouldInitializeDefinition =
      this.isEditing || this.args.job?._newDefinition === undefined;
    const shouldInitializeVariables =
      this.isEditing || this.args.job?._newDefinitionVariables === undefined;

    if (shouldInitializeDefinition && this.definition) {
      if (isEmberDataModel) {
        scheduleOnce('afterRender', this, this.setDefinitionOnModel);
      } else {
        this.setDefinitionOnModel();
      }
    }

    if (shouldInitializeVariables && this.args.variables) {
      const variables = jsonToHcl(this.args.variables.flags).concat(
        this.args.variables.literal,
      );

      if (isEmberDataModel) {
        scheduleOnce(
          'afterRender',
          this,
          this.setDefinitionVariablesOnModel,
          variables,
        );
      } else {
        this.setDefinitionVariablesOnModel(variables);
      }
    }
  }

  get isEditing() {
    return ['new', 'edit'].includes(this.args.context);
  }

  setDefinitionOnModel = () => {
    if (
      !this.args.job ||
      this.args.job.isDestroying ||
      this.args.job.isDestroyed
    ) {
      return;
    }

    const definition = this.definition;

    if (this.args.job._newDefinition !== definition) {
      this.args.job.set('_newDefinition', definition);
    }
  };

  setDefinitionVariablesOnModel = (variables) => {
    if (
      !this.args.job ||
      this.args.job.isDestroying ||
      this.args.job.isDestroyed
    ) {
      return;
    }

    if (this.args.job._newDefinitionVariables !== variables) {
      this.args.job.set('_newDefinitionVariables', variables);
    }
  };

  edit = () => {
    this.setDefinitionOnModel();
    this.args.onToggleEdit(true);
  };

  onCancel = () => {
    this.args.onToggleEdit(false);
  };

  get stage() {
    if (this.planOutput) return 'review';
    if (this.isEditing) return 'edit';
    return 'read';
  }

  @localStorageProperty('nomadMessageJobPlan', true) shouldShowPlanMessage;
  @localStorageProperty('nomadShouldWrapCode', false) shouldWrapCode;

  dismissPlanMessage = () => {
    this.shouldShowPlanMessage = false;
  };

  plan = task({ drop: true }, async () => {
    this.reset();

    try {
      await this.args.job.parse();
    } catch (err) {
      this.onError(err, 'parse', 'parse jobs');
      return;
    }

    try {
      const plan = await this.args.job.plan();
      this.planOutput = plan;
    } catch (err) {
      this.onError(err, 'plan', 'plan jobs');
    }
  });

  submit = task(async () => {
    try {
      if (this.args.context === 'new') {
        await this.args.job.run();
      } else {
        await this.args.job.update(this.args.format);
      }

      const id = this.args.job.plainId;
      const namespace = this.args.job.belongsTo('namespace').id() || 'default';

      this.reset();
      this.args.onSubmit(id, namespace);
    } catch (err) {
      this.onError(err, 'run', 'submit jobs');
      this.planOutput = null;
    }
  });

  onError(err, type, actionMsg) {
    const error = messageFromAdapterError(err, actionMsg);
    this.error = { message: error, type };
    this.scrollToError();
  }

  reset = () => {
    this.planOutput = null;
    this.error = null;
  };

  scrollToError() {
    if (!this.config.get('isTest')) {
      window.scrollTo(0, 0);
    }
  }

  updateCode = (value, _codemirror, type = 'job') => {
    if (!this.args.job.isDestroying && !this.args.job.isDestroyed) {
      if (type === 'hclVariables') {
        this.args.job.set('_newDefinitionVariables', value);
      } else {
        this.args.job.set('_newDefinition', value);
      }
    }
  };

  toggleWrap = () => {
    this.shouldWrapCode = !this.shouldWrapCode;
  };

  uploadJobSpec = (event) => {
    const reader = new FileReader();
    reader.onload = () => {
      this.updateCode(reader.result);
    };

    const [file] = event.target.files;
    reader.readAsText(file);
  };

  handleSaveAsFile = async () => {
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
  };

  get definition() {
    if (this.args.view === 'full-definition') {
      return JSON.stringify(this.args.definition, null, 2);
    }

    return this.args.specification;
  }

  get definitionVariables() {
    if (!this.args.variables) {
      return '';
    }

    return jsonToHcl(this.args.variables.flags).concat(
      this.args.variables.literal,
    );
  }

  get data() {
    return {
      cancelable: this.args.cancelable,
      definition: this.definition,
      definitionVariables: this.definitionVariables,
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

  get alertData() {
    return {
      ...this.data,
      error: this.error,
      stage: this.stage,
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

  <template>
    <div ...attributes>
      <JobEditorAlert @data={{this.alertData}} @fns={{this.fns}} />

      {{#if (eq @context "new")}}
        <header class="run-job-header">
          <h1 class="title is-3">Run a job</h1>
          <p>
            Paste or author HCL or JSON to submit to your cluster, or select
            from a list of templates. A plan will be requested before the job is
            submitted. You can also attach a job spec by uploading a job file or
            dragging &amp; dropping a file to the editor.
          </p>
          <HdsButtonSet>
            <label
              class="job-spec-upload hds-button hds-button--color-secondary hds-button--size-medium"
            >
              <div class="hds-button__text">Upload file</div>
              <input
                type="file"
                {{on "change" this.fns.onUpload}}
                accept=".hcl,.json,.nomad"
              />
            </label>
            {{#if
              (can "read variable" path="nomad/job-templates/*" namespace="*")
            }}
              <HdsButton
                @text="Choose from template"
                @color="secondary"
                @route="jobs.run.templates"
                data-test-choose-template
              />
            {{/if}}
          </HdsButtonSet>
        </header>
      {{/if}}
      {{#if (eq this.stage "review")}}
        <JobEditorReview @data={{this.data}} @fns={{this.fns}} />
      {{else if (eq this.stage "edit")}}
        <JobEditorEdit @data={{this.data}} @fns={{this.fns}} />
      {{else}}
        <JobEditorRead @data={{this.data}} @fns={{this.fns}} />
      {{/if}}
    </div>
  </template>
}
