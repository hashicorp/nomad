/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

// @ts-check

export function initialize(application) {
  application.inject('route', 'application', 'service:history');
  application.inject('component', 'history', 'service:history'); // TODO: temp!
}

export default {
  name: 'history',
  initialize,
};
