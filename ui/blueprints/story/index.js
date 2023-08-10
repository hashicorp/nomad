/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

/* eslint-disable ember/no-string-prototype-extensions */
const getPathOption = require('ember-cli-get-component-path-option');
const stringUtil = require('ember-cli-string-utils');
const path = require('path');

module.exports = {
  description: 'generates a story for storybook',

  fileMapTokens: function () {
    let { project } = this;
    return {
      __path__: function () {
        return path.relative(project.root, project.root);
      },
      __markdownname__: function (options) {
        return options.dasherizedModuleName;
      },
      __name__: function (options) {
        return options.dasherizedModuleName;
      },
    };
  },

  locals: function (options) {
    let contents = '';

    return {
      contents: contents,
      path: getPathOption(options),
      header: stringUtil
        .dasherize(options.entity.name)
        .split('-')
        .map((word) => stringUtil.capitalize(word))
        .join(' '),
    };
  },
};
