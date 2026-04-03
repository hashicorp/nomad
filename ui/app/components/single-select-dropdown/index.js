/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import { get } from '@ember/object';
import Component from '@glimmer/component';
import { action } from '@ember/object';

export default class SingleSelectDropdown extends Component {
  get activeOption() {
    return this.args.options.find(
      (item) => get(item, 'key') === this.args.selection
    );
  }

  @action
  setSelection({ key }) {
    this.args.onSelect && this.args.onSelect(key);
  }
}
