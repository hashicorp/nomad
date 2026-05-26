/**
 * Copyright IBM Corp. 2015, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

export default {
  plugins: ['prettier-plugin-ember-template-tag'],
  overrides: [
    {
      files: '*.{js,gjs,ts,gts,mjs,mts,cjs,cts}',
      options: {
        singleQuote: true,
        templateSingleQuote: false,
      },
    },
  ],
};
