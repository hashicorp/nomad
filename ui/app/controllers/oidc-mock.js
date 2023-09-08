/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Controller from '@ember/controller';
import { action } from '@ember/object';
import { inject as service } from '@ember/service';
import Ember from 'ember';

export default class OidcMockController extends Controller {
  @service router;

  queryParams = ['auth_method', 'client_nonce', 'redirect_uri', 'meta'];

  @action
  signIn(fakeAccount) {
    const url = `${this.redirect_uri.split('?')[0]}?code=${
      fakeAccount.accessor
    }&state=success`;
    if (Ember.testing) {
      this.router.transitionTo(url);
    } else {
      window.location = url;
    }
  }

  @action
  failToSignIn() {
    const url = `${this.redirect_uri.split('?')[0]}?state=failure`;
    if (Ember.testing) {
      this.router.transitionTo(url);
    } else {
      window.location = url;
    }
  }
}
