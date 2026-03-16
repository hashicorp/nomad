/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@ember/component';
import { tagName } from '@ember-decorators/component';
import { inject as service } from '@ember/service';
import { action } from '@ember/object';

@tagName('')
export default class ForbiddenMessage extends Component {
  @service token;
  @service store;
  @service router;

  forbiddenOriginPath = null;

  didReceiveAttrs() {
    super.didReceiveAttrs(...arguments);

    const currentURL = this.router.currentURL;
    if (
      !this.forbiddenOriginPath &&
      currentURL &&
      currentURL !== '/settings/tokens'
    ) {
      this.forbiddenOriginPath = currentURL;
    }

    if (currentURL && currentURL !== '/settings/tokens') {
      if (!this.token.postExpiryPath) {
        this.token.postExpiryPath = currentURL;
      }
      if (!this.token.forbiddenReturnPath) {
        this.token.forbiddenReturnPath = currentURL;
      }
    }
  }

  @action
  rememberPostExpiryPath() {
    const currentURL = this.forbiddenOriginPath || this.router.currentURL;

    if (!currentURL || currentURL === '/settings/tokens') {
      return;
    }

    this.token.postExpiryPath = currentURL;
    this.token.forbiddenReturnPath = currentURL;
  }

  get authMethods() {
    return this.store.findAll('auth-method');
  }
}
