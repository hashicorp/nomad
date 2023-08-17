/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

import { Factory } from 'ember-cli-mirage';
import generateResources from '../data/generate-resources';

export default Factory.extend({
  resourceUsage: generateResources,

  _taskNames: () => [], // Set by allocation

  timestamp: () => Date.now() * 1000000,

  tasks() {
    var hash = {};
    this._taskNames.forEach(task => {
      hash[task] = {
        Pids: null,
        ResourceUsage: generateResources(),
        Timestamp: Date.now() * 1000000,
      };
    });
    return hash;
  },
});
