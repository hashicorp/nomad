/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

/* eslint-env node */
'use strict';

var path = require('path');
var Funnel = require('broccoli-funnel');

module.exports = {
  name: 'bulma',

  isDevelopingAddon: function () {
    return true;
  },

  included: function (app) {
    this._super.included.apply(this, arguments);

    // see: https://github.com/ember-cli/ember-cli/issues/3718
    while (typeof app.import !== 'function' && app.app) {
      app = app.app;
    }

    this.bulmaPath = path.dirname(require.resolve('bulma'));
    return app;
  },

  treeForStyles: function () {
    return new Funnel(this.bulmaPath, {
      srcDir: '/',
      destDir: 'app/styles/bulma',
      annotation: 'Funnel (bulma)',
    });
  },
};
