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
import { setupGlobalA11yHooks } from 'ember-a11y-testing/test-support';
// @ts-expect-error: no types for ember-exam
import { start } from 'ember-exam/test-support';

setApplication(Application.create(config.APP));

setup(QUnit.assert);
setupEmberOnerrorValidation();

// Configure ember-a11y-testing per addon instructions:
// https://github.com/ember-a11y/ember-a11y-testing
//
// Automatically run an a11y audit after every interaction helper (visit,
// click, fillIn, render, etc.) so accessibility is verified each time a page
// is visited. Acceptance tests should also call `a11yAudit` explicitly after
// `visit(...)` for an opt-in, assertion-emitting audit per route.
setupGlobalA11yHooks(() => true);

// eslint-disable-next-line @typescript-eslint/no-unsafe-call
start();
