/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';
import { inject as service } from '@ember/service';
import { action } from '@ember/object';
import { task } from 'ember-concurrency';
import messageFromAdapterError from 'nomad-ui/utils/message-from-adapter-error';
import localStorageProperty from 'nomad-ui/utils/properties/local-storage';
import { tracked } from '@glimmer/tracking';

export default class JobEditor extends Component {
  @service store;
  @service config;

  @tracked error = null;
  @tracked planOutput = null;

  get stage() {
    return this.planOutput ? 'plan' : 'editor';
  }

  @localStorageProperty('nomadMessageJobPlan', true) showPlanMessage;

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
        yield this.args.job.update();
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
  updateCode(value) {
    if (!this.args.job.isDestroying && !this.args.job.isDestroyed) {
      this.args.job.set('_newDefinition', value);
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
}
