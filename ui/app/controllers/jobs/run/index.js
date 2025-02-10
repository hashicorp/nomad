/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { getOwner } from '@ember/application';
import Controller from '@ember/controller';
import { action } from '@ember/object';
import { inject as service } from '@ember/service';

export default class RunController extends Controller {
  @service router;

  queryParams = ['template', 'sourceString', 'disregardNameWarning'];

  @action
  handleSaveAsTemplate() {
    getOwner(this)
      .lookup('controller:jobs.run')
      .setTemplate(this.model._newDefinition);
  }

  onSubmit(id, namespace) {
    this.router.transitionTo('jobs.job', `${id}@${namespace || 'default'}`);
  }
}
