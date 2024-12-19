/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Controller from '@ember/controller';
import { action } from '@ember/object';
import { inject as service } from '@ember/service';
import { task } from 'ember-concurrency';

export default class SentinelPoliciesIndexController extends Controller {
  @service router;
  @service notifications;

  @action openPolicy(policy) {
    this.router.transitionTo(
      'administration.sentinel-policies.policy',
      policy.name
    );
  }

  @action goToNewPolicy() {
    this.router.transitionTo('administration.sentinel-policies.new');
  }

  @action goToTemplateGallery() {
    this.router.transitionTo('administration.sentinel-policies.gallery');
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
      {
        key: 'delete',
        label: 'Delete',
      },
    ];
  }

  @task(function* (policy) {
    try {
      yield policy.deleteRecord();
      yield policy.save();

      if (this.store.peekRecord('policy', policy.id)) {
        this.store.unloadRecord(policy);
      }

      this.notifications.add({
        title: `Sentinel policy ${policy.name} successfully deleted`,
        color: 'success',
      });
    } catch (err) {
      this.notifications.add({
        title: 'Error deleting policy',
        color: 'critical',
        sticky: true,
        message: err,
      });

      throw err;
    }
  })
  deletePolicy;
}
