/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import Controller from '@ember/controller';
import { service } from '@ember/service';

export default class SettingsController extends Controller {
  @service keyboard;
  @service token;
  @service system;

  get tokenRecord() {
    return this.token.selfToken;
  }

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
