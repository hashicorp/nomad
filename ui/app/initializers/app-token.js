/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

export function initialize() {
  const application = arguments[1] || arguments[0];

  // Provides the acl token service to all templates
  application.inject('controller', 'token', 'service:token');
  application.inject('component', 'token', 'service:token');
}

export default {
  name: 'app-token',
  initialize,
};
