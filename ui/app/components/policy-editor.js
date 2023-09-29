/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';
import { action } from '@ember/object';
import { inject as service } from '@ember/service';
import { alias } from '@ember/object/computed';

export default class PolicyEditorComponent extends Component {
  @service notifications;
  @service router;
  @service store;

  @alias('args.policy') policy;

  @action updatePolicyRules(value) {
    this.policy.set('rules', value);
  }

  @action updatePolicyName({ target: { value } }) {
    this.policy.set('name', value);
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
      // Because we set the ID for adapter/serialization reasons just before save here,
      // that becomes a barrier to our Unique Name validation. So we explicltly exclude
      // the current policy when checking for uniqueness.
      if (
        this.policy.isNew &&
        this.store
          .peekAll('policy')
          .filter((policy) => policy !== this.policy)
          .findBy('name', this.policy.name)
      ) {
        throw new Error(
          `A policy with name ${this.policy.name} already exists.`
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
          'access-control.policies.policy',
          this.policy.id
        );
      }
    } catch (error) {
      this.notifications.add({
        title: `Error creating Policy ${this.policy.name}`,
        message: error,
        color: 'critical',
        sticky: true,
      });
    }
  }
}
