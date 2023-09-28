/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

// @ts-check

import { action } from '@ember/object';
import { inject as service } from '@ember/service';
import { alias } from '@ember/object/computed';
import Component from '@glimmer/component';
import { tracked } from '@glimmer/tracking';

export default class RoleEditorComponent extends Component {
  @service notifications;
  @service router;
  @service store;

  @alias('args.role') role;

  @tracked rolePolicies = [];

  // when this renders, set up rolePOlicies
  constructor() {
    super(...arguments);
    this.rolePolicies = this.role.policies.toArray() || [];
  }

  @action updateRoleName({ target: { value } }) {
    this.role.set('name', value);
  }

  @action updateRolePolicies(policy, event) {
    let { checked } = event.target;
    if (checked) {
      this.rolePolicies.push(policy);
    } else {
      this.rolePolicies = this.rolePolicies.filter((p) => p !== policy);
    }
  }

  @action async save(e) {
    if (e instanceof Event) {
      e.preventDefault(); // code-mirror "command+enter" submits the form, but doesnt have a preventDefault()
    }
    try {
      const nameRegex = '^[a-zA-Z0-9-]{1,128}$';
      if (!this.role.name?.match(nameRegex)) {
        throw new Error(
          `Role name must be 1-128 characters long and can only contain letters, numbers, and dashes.`
        );
      }

      const shouldRedirectAfterSave = this.role.isNew;

      if (this.role.isNew && this.store.peekRecord('role', this.role.name)) {
        throw new Error(`A role with name ${this.role.name} already exists.`);
      }

      // If no policies are selected, throw an error
      if (this.rolePolicies.length === 0) {
        throw new Error(`You must select at least one policy.`);
      }

      this.role.policies = this.rolePolicies;

      await this.role.save();

      this.notifications.add({
        title: 'Role Saved',
        color: 'success',
      });

      if (shouldRedirectAfterSave) {
        this.router.transitionTo('access-control.roles.role', this.role.id);
      }
    } catch (error) {
      this.notifications.add({
        title: `Error creating Role ${this.role.name}`,
        message: error,
        color: 'critical',
        sticky: true,
      });
    }
  }
}
