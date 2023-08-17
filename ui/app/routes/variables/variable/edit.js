/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

import Route from '@ember/routing/route';

export default class VariablesVariableEditRoute extends Route {
  model() {
    return this.modelFor('variables.variable');
  }
}
