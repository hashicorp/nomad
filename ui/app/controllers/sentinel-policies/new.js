/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

// @ts-check
import Controller from '@ember/controller';
import { inject as service } from '@ember/service';
// import { task } from 'ember-concurrency';
import { action } from '@ember/object';

export default class SentinelPoliciesNewController extends Controller {
  @service notifications;
  @service router;
  @service store;

  @action updatePolicyDefinition() {
    console.log('sup');
  }

  @action onToggleWrap() {
    console.log('sup');
  }

  @action onSubmit() {
    console.log('sup');
  }
}
