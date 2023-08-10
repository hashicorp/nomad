/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Route from '@ember/routing/route';

export default class OidcMockRoute extends Route {
  // This route only exists for testing SSO/OIDC flow in development, backed by our mirage server.
  // This route won't load outside of a mirage environment, nor will the model hook here return anything meaningful.
  model() {
    return this.store.findAll('token');
  }
}
