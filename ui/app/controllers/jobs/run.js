/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Controller from '@ember/controller';
import { action } from '@ember/object';
import { tracked } from '@glimmer/tracking';

export default class JobsRunController extends Controller {
  @tracked jsonTemplate = null;

  @action
  setTemplate(template) {
    this.jsonTemplate = template;
  }
}
