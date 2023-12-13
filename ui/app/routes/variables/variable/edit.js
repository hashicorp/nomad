/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Route from '@ember/routing/route';

export default class VariablesVariableEditRoute extends Route {
  model() {
    return this.modelFor('variables.variable');
  }
}
