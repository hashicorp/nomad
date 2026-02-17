/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';
import { action } from '@ember/object';

export default class AppBreadcrumbsComponent extends Component {
  @action
  isOneCrumbUp(iter = 0, totalNum = 0) {
    return iter === totalNum - 2;
  }
}
