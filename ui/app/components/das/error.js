/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';
import { action } from '@ember/object';

export default class DasErrorComponent extends Component {
  @action
  dismissClicked() {
    this.args.proceed({ manuallyDismissed: true });
  }
}
