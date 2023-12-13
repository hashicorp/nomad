/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';
import { action } from '@ember/object';
import { inject as service } from '@ember/service';
import { alias } from '@ember/object/computed';

export default class PolicyEditorComponent extends Component {
  @service flashMessages;
  @service router;
  @service store;

  @alias('args.policy') policy;

  @action updatePolicyRules(value) {
    this.policy.set('rules', value);
  }

  @action async save(e) {
    if (e instanceof Event) {
      e.preventDefault(); // code-mirror "command+enter" submits the form, but doesnt have a preventDefault()
    }
    try {
      const nameRegex = '^[a-zA-Z0-9-]{1,128}$';
      if (!this.policy.name?.match(nameRegex)) {
        throw new Error(
          `Policy name must be 1-128 characters long and can only contain letters, numbers, and dashes.`
        );
      }

      const shouldRedirectAfterSave = this.policy.isNew;

      if (
        this.policy.isNew &&
        this.store.peekRecord('policy', this.policy.name)
      ) {
        throw new Error(
          `A policy with name ${this.policy.name} already exists.`
        );
      }

      this.policy.id = this.policy.name;

      await this.policy.save();

      this.flashMessages.add({
        title: 'Policy Saved',
        type: 'success',
        destroyOnClick: false,
        timeout: 5000,
      });

      if (shouldRedirectAfterSave) {
        this.router.transitionTo('policies.policy', this.policy.id);
      }
    } catch (error) {
      this.flashMessages.add({
        title: `Error creating Policy ${this.policy.name}`,
        message: error,
        type: 'error',
        destroyOnClick: false,
        sticky: true,
      });
    }
  }
}
