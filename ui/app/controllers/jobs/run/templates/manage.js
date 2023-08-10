/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Controller from '@ember/controller';
import { inject as service } from '@ember/service';
import { tracked } from '@glimmer/tracking';
import { task } from 'ember-concurrency';

export default class JobsRunTemplatesManageController extends Controller {
  @service notifications;
  @service router;

  get templates() {
    return [...this.model.variables.toArray(), ...this.model.default];
  }

  @tracked selectedTemplate = null;

  columns = ['name', 'namespace', 'description', 'delete'].map((column) => {
    return {
      key: column,
      label: `${column.charAt(0).toUpperCase()}${column.substring(1)}`,
    };
  });

  formatTemplateLabel(path) {
    return path.split('nomad/job-templates/')[1];
  }

  @task(function* (model) {
    try {
      yield model.destroyRecord();
      this.notifications.add({
        title: 'Job template deleted',
        message: `${model.path} successfully deleted`,
        color: 'success',
      });
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
