/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Controller from '@ember/controller';
import { action } from '@ember/object';
import { tracked } from '@glimmer/tracking';

export default class JobsRunTemplatesIndexController extends Controller {
  @tracked selectedTemplate = null;

  get templates() {
    return [...this.model.variables.toArray(), ...this.model.default];
  }

  @action
  onChange(e) {
    this.selectedTemplate = e.target.id;
  }
}
