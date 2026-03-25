/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';
import { tracked } from '@glimmer/tracking';
import { on } from '@ember/modifier';
import { hash } from '@ember/helper';
import { service } from '@ember/service';
import can from 'ember-can/helpers/can';
import {
  HdsButton,
  HdsFormTextInputField,
} from '@hashicorp/design-system-components/components';
import autofocus from 'nomad-ui/modifiers/autofocus';
import codeMirror from 'nomad-ui/modifiers/code-mirror';
import messageFromAdapterError from 'nomad-ui/utils/message-from-adapter-error';
import { not } from 'ember-truth-helpers';

export default class NamespaceEditor extends Component {
  @service notifications;
  @service router;
  @service store;
  @service abilities;

  @tracked JSONError = null;
  @tracked definitionString = this.definitionStringFromNamespace(
    this.args.namespace,
  );

  updateNamespaceName = ({ target: { value } }) => {
    this.args.namespace.set('name', value);
  };

  updateNamespaceDefinition = (value) => {
    this.JSONError = null;
    this.definitionString = value;

    try {
      JSON.parse(this.definitionString);
    } catch {
      this.JSONError = 'Invalid JSON';
    }
  };

  save = async (event) => {
    event?.preventDefault?.();

    const namespace = this.args.namespace;

    try {
      this.deserializeDefinitionJson(JSON.parse(this.definitionString));

      const nameRegex = '^[a-zA-Z0-9-]{1,128}$';
      if (!namespace.name?.match(nameRegex)) {
        throw new Error(
          'Namespace name must be 1-128 characters long and can only contain letters, numbers, and dashes.',
        );
      }

      const shouldRedirectAfterSave = namespace.isNew;

      if (
        namespace.isNew &&
        this.store
          .peekAll('namespace')
          .filter((existingNamespace) => existingNamespace !== namespace)
          .find(el => el.name === namespace.name)
      ) {
        throw new Error(
          `A namespace with name ${namespace.name} already exists.`,
        );
      }

      namespace.set('id', namespace.name);
      await namespace.save();

      this.notifications.add({
        title: 'Namespace Saved',
        color: 'success',
      });

      if (shouldRedirectAfterSave) {
        this.router.transitionTo(
          'administration.namespaces.acl-namespace',
          namespace.name,
        );
      }
    } catch (err) {
      const title = `Error ${
        namespace.isNew ? 'creating' : 'updating'
      } Namespace ${namespace.name}`;

      const message = err.errors?.length
        ? messageFromAdapterError(err)
        : err.message || 'Unknown Error';

      this.notifications.add({
        title,
        message,
        color: 'critical',
        sticky: true,
      });
    }
  };

  definitionStringFromNamespace(namespace) {
    const capabilities = namespace.capabilities
      ? {
          DisabledTaskDrivers:
            namespace.capabilities.DisabledTaskDrivers?.toArray?.() ||
            namespace.capabilities.DisabledTaskDrivers ||
            [],
          EnabledTaskDrivers:
            namespace.capabilities.EnabledTaskDrivers?.toArray?.() ||
            namespace.capabilities.EnabledTaskDrivers ||
            [],
        }
      : undefined;

    const nodePoolConfiguration = namespace.nodePoolConfiguration
      ? {
          Default: namespace.nodePoolConfiguration.Default,
          Allowed:
            namespace.nodePoolConfiguration.Allowed?.toArray?.() ||
            namespace.nodePoolConfiguration.Allowed ||
            null,
          Disallowed:
            namespace.nodePoolConfiguration.Disallowed?.toArray?.() ||
            namespace.nodePoolConfiguration.Disallowed ||
            null,
        }
      : null;

    const definitionHash = {};
    definitionHash.Description = namespace.description;
    definitionHash.Capabilities = capabilities;
    definitionHash.Meta = namespace.meta;

    if (this.abilities.can('configure-in-namespace node-pool')) {
      definitionHash.NodePoolConfiguration = nodePoolConfiguration;
    }

    if (this.abilities.can('configure-in-namespace quota')) {
      definitionHash.Quota = namespace.quota;
    }

    return JSON.stringify(definitionHash, null, 4);
  }

  deserializeDefinitionJson(definitionHash) {
    const namespace = this.args.namespace;

    namespace.set('description', definitionHash.Description);
    namespace.set('meta', definitionHash.Meta);

    const capabilities = this.store.createFragment(
      'ns-capabilities',
      definitionHash.Capabilities,
    );
    namespace.set('capabilities', capabilities);

    if (this.abilities.can('configure-in-namespace node-pool')) {
      const npConfig = definitionHash.NodePoolConfiguration || {};

      if (!('Allowed' in npConfig)) {
        npConfig.Allowed = null;
      }

      if (!('Disallowed' in npConfig)) {
        npConfig.Disallowed = null;
      }

      const nodePoolConfiguration = this.store.createFragment(
        'ns-node-pool-configuration',
        npConfig,
      );

      namespace.set('nodePoolConfiguration', nodePoolConfiguration);
    }

    if (this.abilities.can('configure-in-namespace quota')) {
      namespace.set('quota', definitionHash.Quota);
    }
  }

  <template>
    <form
      class="acl-form namespace-editor"
      autocomplete="off"
      {{on "submit" this.save}}
      ...attributes
    >
      {{#if @namespace.isNew}}
        <HdsFormTextInputField
          @isRequired={{true}}
          data-test-namespace-name-input
          @value={{@namespace.name}}
          {{on "input" this.updateNamespaceName}}
          {{autofocus ignore=(not @namespace.isNew)}}
          as |F|
        >
          <F.Label>Name</F.Label>
        </HdsFormTextInputField>
      {{/if}}

      <div class="boxed-section">
        <div class="boxed-section-head">
          Definition
        </div>
        <div class="boxed-section-body is-full-bleed">
          <div
            class="namespace-editor-wrapper boxed-section-body is-full-bleed
              {{if this.JSONError 'error'}}"
          >
            <div
              class="namespace-editor"
              data-test-namespace-editor
              {{codeMirror
                screenReaderLabel="Namespace definition"
                theme="hashi"
                mode="javascript"
                content=this.definitionString
                onUpdate=this.updateNamespaceDefinition
                autofocus=false
                extraKeys=(hash Cmd-Enter=this.save)
              }}
            />
            {{#if this.JSONError}}
              <p class="help is-danger">
                {{this.JSONError}}
              </p>
            {{/if}}
          </div>
        </div>
      </div>

      <footer>
        {{#if (can "update namespace")}}
          <HdsButton
            @text="Save Namespace"
            @color="primary"
            disabled={{this.JSONError}}
            {{on "click" this.save}}
            data-test-save-namespace
          />
        {{/if}}
      </footer>
    </form>
  </template>
}
