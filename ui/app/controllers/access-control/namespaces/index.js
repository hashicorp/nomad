/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Controller from '@ember/controller';
import { action } from '@ember/object';
import { inject as service } from '@ember/service';

export default class AccessControlNamespacesIndexController extends Controller {
  @service router;
  @service notifications;
  @service can;

  @action openNamespace(namespace) {
    this.router.transitionTo(
      'access-control.namespaces.acl-namespace',
      namespace.name
    );
  }

  @action goToNewNamespace() {
    this.router.transitionTo('access-control.namespaces.new');
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
    ];
  }
}
