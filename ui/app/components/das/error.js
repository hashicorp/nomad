/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

import Component from '@glimmer/component';
import { action } from '@ember/object';

export default class DasErrorComponent extends Component {
  @action
  dismissClicked() {
    this.args.proceed({ manuallyDismissed: true });
  }
}
