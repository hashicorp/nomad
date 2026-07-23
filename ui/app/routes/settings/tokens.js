/**
 * Copyright IBM Corp. 2015, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import Route from '@ember/routing/route';
import { service } from '@ember/service';
import RSVP from 'rsvp';

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
    // use an RSVP.hash for the model so usages of
    // authMethods are properly recomputed.
    return RSVP.hash({
      authMethods: this.store.findAll('auth-method'),
    });
  }
}
