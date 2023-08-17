/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

import Component from '@glimmer/component';

export default class AppBreadcrumbsComponent extends Component {
  isOneCrumbUp(iter = 0, totalNum = 0) {
    return iter === totalNum - 2;
  }
}
