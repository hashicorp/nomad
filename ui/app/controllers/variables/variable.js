/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Controller from '@ember/controller';

export default class VariablesVariableController extends Controller {
  get breadcrumbs() {
    let crumbs = [];
    let id = decodeURI(this.params.id.split('@').slice(0, -1).join('@')); // remove namespace
    let namespace = this.params.id.split('@').slice(-1)[0];
    id.split('/').reduce((m, n) => {
      crumbs.push({
        label: n,
        args:
          m + n === id // If the last crumb, link to the var itself
            ? [`variables.variable`, `${m + n}@${namespace}`]
            : [`variables.path`, m + n],
      });
      return m + n + '/';
    }, []);
    return crumbs;
  }
}
