/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

// @ts-check
import Route from '@ember/routing/route';
import { inject as service } from '@ember/service';

export default class SettingsTokensRoute extends Route {
  @service store;
  model() {
    return {
      authMethods: this.store.findAll('auth-method'),
    };
  }
}
