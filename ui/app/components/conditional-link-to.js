/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

import Component from '@glimmer/component';

export default class ConditionalLinkToComponent extends Component {
  get query() {
    return this.args.query || {};
  }
}
