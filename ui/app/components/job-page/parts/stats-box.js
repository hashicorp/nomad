/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';
import { inject as service } from '@ember/service';
import { marked } from 'marked';
import { htmlSafe } from '@ember/template';

export default class StatsBox extends Component {
  @service system;

  get description() {
    if (!this.args.job.ui?.Description) {
      return null;
    }
    // Put <br /> on newlines, use github-flavoured-markdown.
    marked.use({
      gfm: true,
      breaks: true,
    });
    return htmlSafe(marked.parse(this.args.job.ui.Description));
  }
}
