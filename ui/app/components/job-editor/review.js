/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';
import { htmlSafe } from '@ember/template';

export default class JobEditorReviewComponent extends Component {
  // Slightly formats the warning string to be more readable
  get warnings() {
    return htmlSafe(
      (this.args.data.planOutput.warnings || '')
        .replace(/\n/g, '<br>')
        .replace(/\t/g, '&nbsp;&nbsp;&nbsp;&nbsp;')
    );
  }
}
