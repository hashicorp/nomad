/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

// @ts-check

import { action } from '@ember/object';
import { inject as service } from '@ember/service';
import { alias } from '@ember/object/computed';
import Component from '@glimmer/component';

export default class NamespaceEditorComponent extends Component {
  @service notifications;
  @service router;
  @service store;

  @alias('args.namespace') namespace;

  @action updateNamespaceName({ target: { value } }) {
    this.namespace.set('name', value);
  }

  @action async save(e) {
    if (e instanceof Event) {
      e.preventDefault(); // code-mirror "command+enter" submits the form, but doesnt have a preventDefault()
    }
    try {
      // TODO: Get correct regex
      const nameRegex = '^[a-zA-Z0-9-]{1,128}$';
      if (!this.namespace.name?.match(nameRegex)) {
        throw new Error(
          `Namespace name must be 1-128 characters long and can only contain letters, numbers, and dashes.`
        );
      }

      const shouldRedirectAfterSave = this.namespace.isNew;

      if (
        this.namespace.isNew &&
        this.store.peekRecord('namespace', this.namespace.name)
      ) {
        throw new Error(
          `A namespace with name ${this.namespace.name} already exists.`
        );
      }

      await this.namespace.save();

      this.notifications.add({
        title: 'Namespace Saved',
        color: 'success',
      });

      if (shouldRedirectAfterSave) {
        this.router.transitionTo(
          'access-control.namespaces.namespace',
          this.namespace.name
        );
      }
    } catch (error) {
      this.notifications.add({
        title: `Error creating Namespace ${this.namespace.name}`,
        message: error,
        color: 'critical',
        sticky: true,
      });
    }
  }
}
