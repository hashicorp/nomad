/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

// @ts-check

import { action } from '@ember/object';
import { inject as service } from '@ember/service';
import { alias } from '@ember/object/computed';
import { tracked } from '@glimmer/tracking';
import Component from '@glimmer/component';
import messageFromAdapterError from 'nomad-ui/utils/message-from-adapter-error';

export default class NamespaceEditorComponent extends Component {
  @service notifications;
  @service router;
  @service store;
  @service can;

  @alias('args.namespace') namespace;

  @tracked JSONError = null;
  @tracked definitionString = this.definitionStringFromNamespace(
    this.args.namespace
  );

  @action updateNamespaceName({ target: { value } }) {
    this.namespace.set('name', value);
  }

  @action updateNamespaceDefinition(value) {
    this.JSONError = null;
    this.definitionString = value;

    try {
      JSON.parse(this.definitionString);
    } catch (error) {
      this.JSONError = 'Invalid JSON';
    }
  }

  @action async save(e) {
    if (e instanceof Event) {
      e.preventDefault(); // code-mirror "command+enter" submits the form, but doesnt have a preventDefault()
    }
    try {
      this.deserializeDefinitionJson(JSON.parse(this.definitionString));

      const nameRegex = '^[a-zA-Z0-9-]{1,128}$';
      if (!this.namespace.name?.match(nameRegex)) {
        throw new Error(
          `Namespace name must be 1-128 characters long and can only contain letters, numbers, and dashes.`
        );
      }

      const shouldRedirectAfterSave = this.namespace.isNew;

      if (
        this.namespace.isNew &&
        this.store
          .peekAll('namespace')
          .filter((namespace) => namespace !== this.namespace)
          .findBy('name', this.namespace.name)
      ) {
        throw new Error(
          `A namespace with name ${this.namespace.name} already exists.`
        );
      }

      this.namespace.set('id', this.namespace.name);
      await this.namespace.save();

      this.notifications.add({
        title: 'Namespace Saved',
        color: 'success',
      });

      if (shouldRedirectAfterSave) {
        this.router.transitionTo(
          'access-control.namespaces.acl-namespace',
          this.namespace.name
        );
      }
    } catch (err) {
      let title = `Error ${
        this.namespace.isNew ? 'creating' : 'updating'
      } Namespace ${this.namespace.name}`;

      let message = err.errors?.length
        ? messageFromAdapterError(err)
        : err.message || 'Unknown Error';

      this.notifications.add({
        title,
        message,
        color: 'critical',
        sticky: true,
      });
    }
  }

  definitionStringFromNamespace(namespace) {
    let definitionHash = {};
    definitionHash['Description'] = namespace.description;
    definitionHash['Capabilities'] = namespace.capabilities;
    definitionHash['Meta'] = namespace.meta;

    if (this.can.can('configure-in-namespace node-pool')) {
      definitionHash['NodePoolConfiguration'] = namespace.nodePoolConfiguration;
    }

    if (this.can.can('configure-in-namespace quota')) {
      definitionHash['Quota'] = namespace.quota;
    }

    return JSON.stringify(definitionHash, null, 4);
  }

  deserializeDefinitionJson(definitionHash) {
    this.namespace.set('description', definitionHash['Description']);
    this.namespace.set('meta', definitionHash['Meta']);

    let capabilities = this.store.createFragment(
      'ns-capabilities',
      definitionHash['Capabilities']
    );
    this.namespace.set('capabilities', capabilities);

    if (this.can.can('configure-in-namespace node-pool')) {
      let npConfig = definitionHash['NodePoolConfiguration'];
      this.store.create;

      // If we don't manually set this to null, removing
      // the keys wont update the data framgment, which we want
      if (!('Allowed' in npConfig)) {
        npConfig['Allowed'] = null;
      }

      if (!('Disallowed' in npConfig)) {
        npConfig['Disallowed'] = null;
      }

      // Create node pool config fragment
      let nodePoolConfiguration = this.store.createFragment(
        'ns-node-pool-configuration',
        npConfig
      );

      this.namespace.set('nodePoolConfiguration', nodePoolConfiguration);
    }

    if (this.can.can('configure-in-namespace quota')) {
      this.namespace.set('quota', definitionHash['Quota']);
    }
  }
}
