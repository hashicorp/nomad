/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import Inflector from 'ember-inflector';

export function initialize() {
  const inflector = Inflector.inflector;

  // Tell the inflector that the plural of "quota" is "quotas"
  inflector.irregular('quota', 'quotas');
}

export default {
  name: 'custom-inflector-rules',
  initialize,
};
