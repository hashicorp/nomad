/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';

export default class ConditionalLinkToComponent extends Component {
  get query() {
    return this.args.query || {};
  }
}
