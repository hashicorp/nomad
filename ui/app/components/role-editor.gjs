/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';
import { tracked } from '@glimmer/tracking';
import { array, fn, hash } from '@ember/helper';
import { on } from '@ember/modifier';
import { LinkTo } from '@ember/routing';
import { service } from '@ember/service';
import { not } from 'ember-truth-helpers';
import {
  HdsButton,
  HdsFormTextInputField,
  HdsTable,
} from '@hashicorp/design-system-components/components';
import can from 'ember-can/helpers/can';
import autofocus from 'nomad-ui/modifiers/autofocus';
import messageFromAdapterError from 'nomad-ui/utils/message-from-adapter-error';

export default class RoleEditor extends Component {
  @service notifications;
  @service router;
  @service store;

  @tracked rolePolicies = [];

  constructor() {
    super(...arguments);
    this.rolePolicies = [...this.args.role.policies] || [];
  }

  updateRoleName = ({ target: { value } }) => {
    this.args.role.set('name', value);
  };

  updateRoleDescription = ({ target: { value } }) => {
    this.args.role.set('description', value);
  };

  updateRolePolicies = (policy, event) => {
    const { checked } = event.target;
    if (checked) {
      if (!this.rolePolicies.some((item) => item?.name === policy?.name)) {
        this.rolePolicies = [...this.rolePolicies, policy];
      }
    } else {
      this.rolePolicies = this.rolePolicies.filter(
        (item) => item?.name !== policy?.name,
      );
    }
  };

  hasPolicySelected = (policyName) =>
    this.rolePolicies.some((policy) => policy?.name === policyName);

  save = async (event) => {
    event?.preventDefault?.();

    const role = this.args.role;

    try {
      const nameRegex = '^[a-zA-Z0-9-]{1,128}$';
      if (!role.name?.match(nameRegex)) {
        throw new Error(
          'Role name must be 1-128 characters long and can only contain letters, numbers, and dashes.',
        );
      }

      const shouldRedirectAfterSave = role.isNew;

      if (role.isNew && this.store.peekRecord('role', role.name)) {
        throw new Error(`A role with name ${role.name} already exists.`);
      }

      if (this.rolePolicies.length === 0) {
        throw new Error('You must select at least one policy.');
      }

      role.policies = this.rolePolicies;

      await role.save();

      this.notifications.add({
        title: 'Role Saved',
        color: 'success',
      });

      if (shouldRedirectAfterSave) {
        this.router.transitionTo('administration.roles.role', role.id);
      }
    } catch (err) {
      const message = err.errors?.length
        ? messageFromAdapterError(err)
        : err.message || 'Unknown Error';

      this.notifications.add({
        title: `Error creating Role ${role.name}`,
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
      <HdsFormTextInputField
        @isRequired={{true}}
        data-test-role-name-input
        @value={{@role.name}}
        {{on "input" this.updateRoleName}}
        {{autofocus ignore=(not @role.isNew)}}
        as |F|
      >
        <F.Label>Role Name</F.Label>
      </HdsFormTextInputField>

      <div>
        <label>
          <span>
            Description (optional)
          </span>
          <input
            data-test-role-description-input
            type="text"
            value={{@role.description}}
            class="input"
            {{on "input" this.updateRoleDescription}}
          />
        </label>
      </div>

      <div>
        <label>
          Policies
        </label>
        <HdsTable
          @caption="A list of policies available to this role"
          class="acl-table"
          @model={{@policies}}
          @columns={{array
            (hash key="selected" width="80px")
            (hash key="name" label="Name" isSortable=true)
            (hash key="description" label="Description")
            (hash key="definition" label="View Policy Definition")
          }}
          @sortBy="name"
          data-test-role-policies
        >
          <:body as |B|>
            <B.Tr>
              <B.Td class="selection-checkbox">
                <label>
                  <input
                    type="checkbox"
                    checked={{this.hasPolicySelected B.data.name}}
                    {{on "change" (fn this.updateRolePolicies B.data)}}
                  />
                </label>
              </B.Td>
              <B.Td data-test-policy-name>{{B.data.name}}</B.Td>
              <B.Td>{{B.data.description}}</B.Td>
              <B.Td>
                <LinkTo
                  @route="administration.policies.policy"
                  @model={{B.data.name}}
                >
                  View Policy Definition
                </LinkTo>
              </B.Td>
            </B.Tr>
          </:body>
        </HdsTable>
      </div>

      <footer>
        {{#if (can "update role")}}
          <HdsButton
            @text="Save Role"
            @color="primary"
            {{on "click" this.save}}
            data-test-save-role
          />
        {{/if}}
      </footer>
    </form>
  </template>
}
