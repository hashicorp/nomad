/**
 * Copyright IBM Corp. 2015, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import { assert } from '@ember/debug';
import { service } from '@ember/service';
import Component from '@glimmer/component';

export default class Breadcrumb extends Component {
  @service breadcrumbs;

  constructor() {
    super(...arguments);
    assert('Provide a valid breadcrumb argument', this.args.crumb);
    this.register();
  }

  register() {
    this.breadcrumbs.registerBreadcrumb(this);
  }

  deregister() {
    this.breadcrumbs.deregisterBreadcrumb(this);
  }

  willDestroy() {
    super.willDestroy();
    this.deregister();
  }
}
