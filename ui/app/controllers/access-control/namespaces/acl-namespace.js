/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

// @ts-check
import Controller from '@ember/controller';
import { inject as service } from '@ember/service';
import { task } from 'ember-concurrency';
import rollbackWithoutChangedAttrs from 'nomad-ui/utils/rollback-without-changed-attrs';
import messageFromAdapterError from 'nomad-ui/utils/message-from-adapter-error';

export default class AccessControlNamespacesAclNamespaceController extends Controller {
  @service notifications;
  @service router;
  @service store;

  @task(function* () {
    try {
      yield this.model.destroyRecord();
      this.notifications.add({
        title: 'Namespace Deleted',
        color: 'success',
        type: `success`,
        destroyOnClick: false,
      });
      this.router.transitionTo('access-control.namespaces');
    } catch (err) {
      // A failed delete resulted in errors when you then navigated away and back
      // to the show page rollbackWithoutChangedAttrs fixes it, but there might
      // be a more idiomatic way
      rollbackWithoutChangedAttrs(this.model);

      let message = err.errors?.length
        ? messageFromAdapterError(err)
        : err.message || 'Unknown Error';

      this.notifications.add({
        title: `Error deleting Namespace ${this.model.name}`,
        message,
        color: 'critical',
        sticky: true,
      });
    }
  })
  deleteNamespace;
}
