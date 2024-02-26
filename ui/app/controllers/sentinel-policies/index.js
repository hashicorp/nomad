/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Controller from '@ember/controller';
import { action } from '@ember/object';
import { inject as service } from '@ember/service';

export default class SentinelPoliciesIndexController extends Controller {
  @service router;
  @service notifications;
  @service can;

  @action openPolicy(policy) {
    this.router.transitionTo('sentinel-policies.policy', policy.name);
  }

  @action goToNewPolicy() {
    this.router.transitionTo('sentinel-policies.policy.new');
  }

  get columns() {
    return [
      {
        key: 'name',
        label: 'Name',
        isSortable: true,
      },
      {
        key: 'description',
        label: 'Description',
      },
      {
        key: 'enforcementLevel',
        label: 'Enforcement Level',
        isSortable: true,
      },
    ];
  }
}
