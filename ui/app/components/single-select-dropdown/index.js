/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';
import { action } from '@ember/object';

export default class SingleSelectDropdown extends Component {
  get activeOption() {
    return this.args.options.findBy('key', this.args.selection);
  }

  @action
  setSelection({ key }) {
    this.args.onSelect && this.args.onSelect(key);
  }
}
