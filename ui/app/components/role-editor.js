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

  @action updateRolePolicies(policy, event) {
    let { value, checked } = event.target;
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

      // TODO: this check currently fires on unsuccessful save. Dont do that!
      if (this.role.isNew && this.store.peekRecord('role', this.role.name)) {
        throw new Error(`A role with name ${this.role.name} already exists.`);
      }

      this.role.id = this.role.name;
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
