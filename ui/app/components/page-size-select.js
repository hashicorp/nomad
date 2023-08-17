/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

import Component from '@ember/component';
import { inject as service } from '@ember/service';

export default class PageSizeSelect extends Component {
  @service userSettings;

  tagName = '';
  pageSizeOptions = [10, 25, 50];

  onChange() {}
}
