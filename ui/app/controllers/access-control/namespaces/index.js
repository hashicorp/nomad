/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Controller from '@ember/controller';
import { action } from '@ember/object';
import { inject as service } from '@ember/service';
import { task } from 'ember-concurrency';

export default class AccessControlNamespacesIndexController extends Controller {
  @service router;
  @service notifications;
  @service can;

  @action openNamespace(namespace) {
    this.router.transitionTo(
      'access-control.namespaces.namespace',
      namespace.name
    );
  }

  @action goToNewNamespace() {
    this.router.transitionTo('access-control.namespaces.new');
  }

  get columns() {
    const defaultColumns = [
      {
        key: 'name',
        label: 'Name',
        isSortable: true,
      },
      {
        key: 'description',
        label: 'Description',
      },
    ];

    // TODO: clean up
    return [...defaultColumns];
  }
  @task(function* (namespace) {
    try {
      yield namespace.deleteRecord();
      yield namespace.save();
      this.notifications.add({
        title: `Namespace ${namespace.name} successfully deleted`,
        color: 'success',
      });
    } catch (err) {
      this.error = {
        title: 'Error deleting namespace',
        description: err,
      };

      throw err;
    }
  })
  deleteNamespace;
}
