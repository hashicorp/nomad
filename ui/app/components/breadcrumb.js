/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { assert } from '@ember/debug';
import { action } from '@ember/object';
import { inject as service } from '@ember/service';
import Component from '@glimmer/component';

export default class Breadcrumb extends Component {
  @service breadcrumbs;

  constructor() {
    super(...arguments);
    assert('Provide a valid breadcrumb argument', this.args.crumb);
    this.register();
  }

  @action register() {
    this.breadcrumbs.registerBreadcrumb(this);
  }

  @action deregister() {
    this.breadcrumbs.deregisterBreadcrumb(this);
  }

  willDestroy() {
    super.willDestroy();
    this.deregister();
  }
}
