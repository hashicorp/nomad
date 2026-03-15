/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import Application from 'nomad-ui/app';
import config from 'nomad-ui/config/environment';
import * as QUnit from 'qunit';
import { setApplication } from '@ember/test-helpers';
import { setup } from 'qunit-dom';
import { setupEmberOnerrorValidation } from 'ember-qunit';
import { start } from 'ember-exam/test-support';

setApplication(Application.create(config.APP));

setup(QUnit.assert);
setupEmberOnerrorValidation();

// Ignore benign ResizeObserver loop errors triggered by HDS components during
// tests by filtering them before QUnit records them as failures.
QUnit.begin(function () {
  const originalOnError = window.onerror;

  window.onerror = function (message, ...args) {
    if (
      typeof message === 'string' &&
      message.includes('ResizeObserver loop')
    ) {
      return true;
    }

    if (originalOnError) {
      return originalOnError.apply(this, [message, ...args]);
    }
  };
});

start();
