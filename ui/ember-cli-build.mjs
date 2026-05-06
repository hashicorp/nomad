/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

'use strict';

import EmberApp from 'ember-cli/lib/broccoli/ember-app.js';
import { createRequire } from 'node:module';

const require = createRequire(import.meta.url);

const environment = EmberApp.env();
const isProd = environment === 'production';
const isTest = environment === 'test';

export default function (defaults) {
  const app = new EmberApp(defaults, {
    emberData: {
      deprecations: {
        // New projects can safely leave this deprecation disabled.
        // If upgrading, to opt-into the deprecated behavior, set this to true and then follow:
        // https://deprecations.emberjs.com/id/ember-data-deprecate-store-extends-ember-object
        // before upgrading to Ember Data 6.0
        DEPRECATE_STORE_EXTENDS_EMBER_OBJECT: false,
      },
    },

    'ember-cli-babel': {
      includePolyfill: isProd,
      enableTypeScriptTransform: true,
    },

    babel: {
      plugins: [
        require.resolve('ember-concurrency/async-arrow-task-transform'),
      ],
    },

    hinting: isTest,
    tests: isTest,
    sassOptions: {
      precision: 4,
      includePaths: [
        './node_modules/bulma',
        './node_modules/xterm/css',
        './node_modules/codemirror/lib',
        './node_modules/@hashicorp/design-system-components/dist/styles',
        './node_modules/ember-basic-dropdown/dist/vendor',
        './node_modules/ember-power-select/dist/vendor',
      ],
    },

    codemirror: {
      modes: ['javascript', 'ruby'],
    },
    // Add options here
  });

  return app.toTree();
}
