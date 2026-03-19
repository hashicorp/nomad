/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';
import { on } from '@ember/modifier';
import { hash } from '@ember/helper';
import { service } from '@ember/service';
import can from 'ember-can/helpers/can';
import {
  HdsButton,
  HdsFormTextInputField,
} from '@hashicorp/design-system-components/components';
import codeMirror from 'nomad-ui/modifiers/code-mirror';
import autofocus from 'nomad-ui/modifiers/autofocus';
import messageFromAdapterError from 'nomad-ui/utils/message-from-adapter-error';

export default class PolicyEditorComponent extends Component {
  @service notifications;
  @service router;
  @service store;

  get policy() {
    return this.args.policy;
  }

  updatePolicyRules = (value) => {
    this.policy?.set?.('rules', value);
  };

  updatePolicyName = ({ target: { value } }) => {
    this.policy?.set?.('name', value);
  };

  updatePolicyDescription = ({ target: { value } }) => {
    this.policy?.set?.('description', value);
  };

  save = async (event) => {
    if (event instanceof Event) {
      // code-mirror "command+enter" submits the form, but may not have preventDefault.
      event.preventDefault();
    }

    try {
      const nameRegex = '^[a-zA-Z0-9-]{1,128}$';
      if (!this.policy?.name?.match(nameRegex)) {
        throw new Error(
          'Policy name must be 1-128 characters long and can only contain letters, numbers, and dashes.',
        );
      }

      const shouldRedirectAfterSave = this.policy.isNew;

      // Because we set the ID for adapter/serialization reasons just before save here,
      // that becomes a barrier to our Unique Name validation. So we explicitly exclude
      // the current policy when checking for uniqueness.
      if (
        this.policy.isNew &&
        this.store
          .peekAll('policy')
          .filter((policy) => policy !== this.policy)
          .findBy('name', this.policy.name)
      ) {
        throw new Error(
          `A policy with name ${this.policy.name} already exists.`,
        );
      }

      this.policy.set('id', this.policy.name);
      await this.policy.save();

      this.notifications.add({
        title: 'Policy Saved',
        color: 'success',
      });

      if (shouldRedirectAfterSave) {
        this.router.transitionTo(
          'administration.policies.policy',
          this.policy.id,
        );
      }
    } catch (err) {
      const message = err.errors?.length
        ? messageFromAdapterError(err)
        : err.message || 'Unknown Error';

      this.notifications.add({
        title: `Error creating Policy ${this.policy?.name}`,
        message,
        color: 'critical',
        sticky: true,
      });
    }
  };

  <template>
    <form class="acl-form" autocomplete="off" {{on "submit" this.save}}>
      {{#if @policy.isNew}}
        <HdsFormTextInputField
          @isRequired={{true}}
          data-test-policy-name-input
          @value={{@policy.name}}
          {{on "input" this.updatePolicyName}}
          {{autofocus}}
          as |F|
        >
          <F.Label>Policy Name</F.Label>
        </HdsFormTextInputField>
      {{/if}}

      <div class="boxed-section">
        <div class="boxed-section-head">
          Policy Definition
        </div>
        <div class="boxed-section-body is-full-bleed">

          <div
            class="policy-editor"
            data-test-policy-editor
            {{codeMirror
              screenReaderLabel="Policy definition"
              theme="hashi"
              mode="ruby"
              content=@policy.rules
              onUpdate=this.updatePolicyRules
              autofocus=false
              extraKeys=(hash Cmd-Enter=this.save)
            }}
          />
        </div>
      </div>

      <div>
        <label>
          <span>
            Description (optional)
          </span>
          <input
            data-test-policy-description
            type="text"
            value={{@policy.description}}
            class="input"
            {{on "input" this.updatePolicyDescription}}
          />
        </label>
      </div>

      <footer>
        {{#if (can "update policy")}}
          <HdsButton
            @text="Save Policy"
            @type="submit"
            data-test-save-policy
            {{on "click" this.save}}
          />
        {{/if}}
      </footer>
    </form>
  </template>
}
