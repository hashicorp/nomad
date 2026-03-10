/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

export function initialize() {
  const application = arguments[1] || arguments[0];

  // Provides the app config to all templates
  application.inject('controller', 'config', 'service:config');
  application.inject('component', 'config', 'service:config');
}

export default {
  name: 'app-config',
  initialize,
};
