/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

import Component from '@glimmer/component';
import { inject as service } from '@ember/service';

export default class Breadcrumbs extends Component {
  @service breadcrumbs;

  get crumbs() {
    return this.breadcrumbs.crumbs;
  }
}
