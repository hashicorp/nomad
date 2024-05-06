/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

// @ts-check
import Controller from '@ember/controller';
import { inject as service } from '@ember/service';
import { alias } from '@ember/object/computed';

export default class SettingsController extends Controller {
  @service keyboard;
  @service token;
  @service system;

  @alias('token.selfToken') tokenRecord;

  // Show sign-in if:
  // - User can't load agent config (meaning ACLs are enabled but they're not signed in)
  // - User can load agent config in and ACLs are enabled (meaning ACLs are enabled and they're signed in)
  // The excluded case here is if there is both an agent config and ACLs are disabled
  get shouldShowProfileLink() {
    return (
      !this.system.agent?.get('config') ||
      this.system.agent?.get('config.ACL.Enabled') === true
    );
  }
}
