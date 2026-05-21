/**
 * Copyright IBM Corp. 2015, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import Application from 'nomad-ui/app';
import config from 'nomad-ui/config/environment';
import * as QUnit from 'qunit';
import { setApplication } from '@ember/test-helpers';
import { setup } from 'qunit-dom';
import { setupEmberOnerrorValidation } from 'ember-qunit';
import {
  setupGlobalA11yHooks,
  setRunOptions,
} from 'ember-a11y-testing/test-support';
// @ts-expect-error: no types for ember-exam
import { start } from 'ember-exam/test-support';

setApplication(Application.create(config.APP));

setup(QUnit.assert);
setupEmberOnerrorValidation();

// Configure ember-a11y-testing per addon instructions:
// https://github.com/ember-a11y/ember-a11y-testing
//
// Rules are scoped here (not silently disabled in a wrapper helper) so the
// configuration is centralized and discoverable. Each suppressed rule should
// be tracked toward eventual remediation rather than hidden indefinitely.
setRunOptions({
  rules: {
    'color-contrast': { enabled: false },
    'heading-order': { enabled: false },
  },
});

// Automatically run an a11y audit after every interaction helper (visit,
// click, fillIn, render, etc.). This ensures accessibility is verified each
// time a page is visited without requiring tests to opt in individually.
setupGlobalA11yHooks(() => true);

// eslint-disable-next-line @typescript-eslint/no-unsafe-call
start();
