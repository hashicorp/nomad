/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

// @ts-check
import Controller from '@ember/controller';
import { inject as service } from '@ember/service';
import { action } from '@ember/object';
import { tracked } from '@glimmer/tracking';
import TEMPLATES from 'nomad-ui/utils/default-sentinel-policy-templates';

export default class SentinelPoliciesNewGalleryController extends Controller {
  @service notifications;
  @service router;
  @service store;
  @tracked selectedTemplate = null;

  get templates() {
    return TEMPLATES;
  }

  @action
  onChange(e) {
    this.selectedTemplate = e.target.id;
  }
}
