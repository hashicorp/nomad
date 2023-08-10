/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Controller from '@ember/controller';
import { inject as service } from '@ember/service';
import { action } from '@ember/object';
import { tracked } from '@glimmer/tracking';
import { task } from 'ember-concurrency';

export default class JobsRunTemplatesController extends Controller {
  @service notifications;
  @service router;
  @service system;

  @tracked formModalActive = false;

  @action
  updateKeyValue(key, value) {
    if (this.model.keyValues.find((kv) => kv.key === key)) {
      this.model.keyValues.find((kv) => kv.key === key).value = value;
    } else {
      this.model.keyValues.pushObject({ key, value });
    }
  }

  @action
  toggleModal() {
    this.formModalActive = !this.formModalActive;
  }

  @action
  async save(e, overwrite = false) {
    if (e.type === 'submit') {
      e.preventDefault();
    }

    try {
      await this.model.save({ adapterOptions: { overwrite } });

      this.notifications.add({
        title: 'Job template saved',
        message: `${this.model.path} successfully editted`,
        color: 'success',
      });

      this.router.transitionTo('jobs.run.templates');
    } catch (e) {
      this.notifications.add({
        title: 'Job template cannot be editted.',
        message: e,
        color: 'critical',
      });
    }
  }

  @task(function* () {
    try {
      yield this.model.destroyRecord();

      this.notifications.add({
        title: 'Job template deleted',
        message: `${this.model.path} successfully deleted`,
        color: 'success',
      });
      this.router.transitionTo('jobs.run.templates.manage');
    } catch (err) {
      this.notifications.add({
        title: `Job template could not be deleted.`,
        message: err,
        color: 'critical',
        sticky: true,
      });
    }
  })
  deleteTemplate;
}
