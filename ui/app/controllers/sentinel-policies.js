/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Controller from '@ember/controller';
import { tracked } from '@glimmer/tracking';
import { action } from '@ember/object';

// The WithNamespaceResetting Mixin uses Controller Injection and requires us to keep this controller around
export default class SentinelPoliciesController extends Controller {
  @tracked selectedTemplate = null;

  @action
  onChange(e) {
    this.selectedTemplate = e.target.id;
  }
}
