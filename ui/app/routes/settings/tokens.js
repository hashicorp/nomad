/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
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
