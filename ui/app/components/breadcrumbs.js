/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';
import { inject as service } from '@ember/service';

export default class Breadcrumbs extends Component {
  @service breadcrumbs;

  get crumbs() {
    return this.breadcrumbs.crumbs;
  }
}
