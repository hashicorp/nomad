/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

// @ts-check

import Component from '@glimmer/component';
import { action } from '@ember/object';
import { inject as service } from '@ember/service';
import { alias } from '@ember/object/computed';
import messageFromAdapterError from 'nomad-ui/utils/message-from-adapter-error';

export default class SentinelPolicyEditorComponent extends Component {
  @service notifications;
  @service router;
  @service store;

  @alias('args.policy') policy;

  @action updatePolicy(value) {
    this.policy.set('policy', value);
  }

  @action updatePolicyName({ target: { value } }) {
    this.policy.set('name', value);
  }

  @action updatePolicyEnforcementLevel({ target: { id } }) {
    this.policy.set('enforcementLevel', id);
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
      if (this.policy.description?.length > 256) {
        throw new Error(
          `Policy description must be under 256 characters long.`
        );
      }

      const shouldRedirectAfterSave = this.policy.isNew;
      // Because we set the ID for adapter/serialization reasons just before save here,
      // that becomes a barrier to our Unique Name validation. So we explicltly exclude
      // the current policy when checking for uniqueness.
      if (
        this.policy.isNew &&
        this.store
          .peekAll('sentinel-policy')
          .filter((policy) => policy !== this.policy)
          .findBy('name', this.policy.name)
      ) {
        throw new Error(
          `A sentinel policy with name ${this.policy.name} already exists.`
        );
      }
      this.policy.set('id', this.policy.name);
      await this.policy.save();

      this.notifications.add({
        title: 'Sentinel Policy Saved',
        color: 'success',
      });

      if (shouldRedirectAfterSave) {
        this.router.transitionTo(
          'administration.sentinel-policies.policy',
          this.policy.name
        );
      }
    } catch (err) {
      let message = err.errors?.length
        ? messageFromAdapterError(err)
        : err.message || 'Unknown Error';

      this.notifications.add({
        title: `Error creating Sentinel Policy ${this.policy.name}`,
        message,
        color: 'critical',
        sticky: true,
      });
    }
  }
}
