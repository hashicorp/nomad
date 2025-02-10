/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

// @ts-check
import Route from '@ember/routing/route';
import { inject as service } from '@ember/service';

export default class SettingsTokensRoute extends Route {
  @service store;
  @service system;
  @service router;

  // before model hook: if there is an agent config, and ACLs are disabled,
  // guard against this route. Redirect the user to the "profile settings" page instead.
  async beforeModel() {
    await this.system.agent;
    if (
      this.system.agent?.get('config') &&
      !this.system.agent?.get('config.ACL.Enabled')
    ) {
      this.router.transitionTo('settings.user-settings');
      return;
    }
  }

  model() {
    return {
      authMethods: this.store.findAll('auth-method'),
    };
  }
}
