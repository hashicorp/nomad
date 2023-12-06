/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

// @ts-check
import Controller from '@ember/controller';
import { inject as service } from '@ember/service';
import { alias } from '@ember/object/computed';
import { task } from 'ember-concurrency';

export default class AccessControlHamespacesHamespaceController extends Controller {
  @service notifications;
  @service router;
  @service store;

  @alias('model.namespace') namespace;

  @task(function* () {
    try {
      yield this.namespace.deleteRecord();
      yield this.namespace.save();
      this.notifications.add({
        title: 'Namespace Deleted',
        color: 'success',
        type: `success`,
        destroyOnClick: false,
      });
      this.router.transitionTo('access-control.namespaces');
    } catch (err) {
      this.notifications.add({
        title: `Error deleting Namespace ${this.namespace.name}`,
        message: err,
        color: 'critical',
        sticky: true,
      });
    }
  })
  deleteNamespace;
}
