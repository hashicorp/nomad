/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

import 'core-js';
import Application from 'nomad-ui/app';
import config from 'nomad-ui/config/environment';
import * as QUnit from 'qunit';
import { setApplication } from '@ember/test-helpers';
import start from 'ember-exam/test-support/start';
import { setup } from 'qunit-dom';
import './helpers/flash-message';
import Ember from 'ember';

setApplication(Application.create(config.APP));

Ember.onerror = function (err) {
  console.log('an onerror event has occurred and is being overridden');
  console.error(err);
  console.log('stringified', JSON.stringify(err));
  console.log('end of onerror event');
  QUnit.assert.ok(false, err);
  return new Error(err);
};

setup(QUnit.assert);

start();
