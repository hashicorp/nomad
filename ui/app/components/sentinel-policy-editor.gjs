/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import { get } from '@ember/object';
import Component from '@glimmer/component';
import { hash } from '@ember/helper';
import { on } from '@ember/modifier';
import { service } from '@ember/service';
import can from 'ember-can/helpers/can';
import { eq } from 'ember-truth-helpers';
import {
  HdsButton,
  HdsFormRadioGroup,
  HdsFormTextInputField,
  HdsLinkInline,
} from '@hashicorp/design-system-components/components';
import autofocus from 'nomad-ui/modifiers/autofocus';
import codeMirror from 'nomad-ui/modifiers/code-mirror';
import messageFromAdapterError from 'nomad-ui/utils/message-from-adapter-error';

export default class SentinelPolicyEditor extends Component {
  @service notifications;
  @service router;
  @service store;

  updatePolicy = (value) => {
    this.args.policy.set('policy', value);
  };

  updatePolicyName = ({ target: { value } }) => {
    this.args.policy.set('name', value);
  };

  updatePolicyDescription = ({ target: { value } }) => {
    this.args.policy.set('description', value);
  };

  updatePolicyEnforcementLevel = ({ target: { id } }) => {
    this.args.policy.set('enforcementLevel', id);
  };

  updatePolicyScope = ({ target: { id } }) => {
    this.args.policy.set('scope', id);
  };

  save = async (event) => {
    event?.preventDefault?.();

    const policy = this.args.policy;

    try {
      const nameRegex = '^[a-zA-Z0-9-]{1,128}$';
      if (!policy.name?.match(nameRegex)) {
        throw new Error(
          'Policy name must be 1-128 characters long and can only contain letters, numbers, and dashes.',
        );
      }
      if (policy.description?.length > 256) {
        throw new Error(
          'Policy description must be under 256 characters long.',
        );
      }

      const shouldRedirectAfterSave = policy.isNew;

      if (
        policy.isNew &&
        this.store
          .peekAll('sentinel-policy')
          .filter((existingPolicy) => existingPolicy !== policy)
          .find(item => get(item, 'name') === policy.name)
      ) {
        throw new Error(
          `A sentinel policy with name ${policy.name} already exists.`,
        );
      }

      policy.set('id', policy.name);
      await policy.save();

      this.notifications.add({
        title: 'Sentinel Policy Saved',
        color: 'success',
      });

      if (shouldRedirectAfterSave) {
        this.router.transitionTo(
          'administration.sentinel-policies.policy',
          policy.name,
        );
      }
    } catch (err) {
      const message = err.errors?.length
        ? messageFromAdapterError(err)
        : err.message || 'Unknown Error';

      this.notifications.add({
        title: `Error creating Sentinel Policy ${policy.name}`,
        message,
        color: 'critical',
        sticky: true,
      });
    }
  };

  <template>
    <form
      class="acl-form"
      autocomplete="off"
      {{on "submit" this.save}}
      ...attributes
    >
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
              content=@policy.policy
              onUpdate=this.updatePolicy
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

      <div>
        <HdsFormRadioGroup
          @layout="horizontal"
          @name="policy-enforcement-level"
          {{on "change" this.updatePolicyEnforcementLevel}}
          as |G|
        >
          <G.Legend>Enforcement Level</G.Legend>
          <G.HelperText>See
            <HdsLinkInline
              @href="https://developer.hashicorp.com/sentinel/docs/concepts/enforcement-levels"
            >Sentinel Policy documentation</HdsLinkInline>
            for more information.</G.HelperText>
          <G.RadioField
            @id="advisory"
            checked={{eq @policy.enforcementLevel "advisory"}}
            data-test-enforcement-level="advisory"
            as |F|
          >
            <F.Label>Advisory</F.Label>
          </G.RadioField>
          <G.RadioField
            @id="soft-mandatory"
            checked={{eq @policy.enforcementLevel "soft-mandatory"}}
            data-test-enforcement-level="soft-mandatory"
            as |F|
          >
            <F.Label>Soft Mandatory</F.Label>
          </G.RadioField>
          <G.RadioField
            @id="hard-mandatory"
            checked={{eq @policy.enforcementLevel "hard-mandatory"}}
            data-test-enforcement-level="hard-mandatory"
            as |F|
          >
            <F.Label>Hard Mandatory</F.Label>
          </G.RadioField>
        </HdsFormRadioGroup>
      </div>

      <div>
        <HdsFormRadioGroup
          @layout="horizontal"
          @name="policy-scope"
          {{on "change" this.updatePolicyScope}}
          as |G|
        >
          <G.Legend>Scope</G.Legend>
          <G.RadioField
            @id="submit-job"
            checked={{eq @policy.scope "submit-job"}}
            data-test-scope="submit-job"
            as |F|
          >
            <F.Label>Submit Job</F.Label>
          </G.RadioField>
          <G.RadioField
            @id="submit-host-volume"
            checked={{eq @policy.scope "submit-host-volume"}}
            data-test-scope="submit-host-volume"
            as |F|
          >
            <F.Label>Submit Host Volume</F.Label>
          </G.RadioField>
          <G.RadioField
            @id="submit-csi-volume"
            checked={{eq @policy.scope "submit-csi-volume"}}
            data-test-scope="submit-csi-volume"
            as |F|
          >
            <F.Label>Submit CSI Volume</F.Label>
          </G.RadioField>
        </HdsFormRadioGroup>
      </div>

      <footer>
        {{#if (can "update sentinel-policy")}}
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
