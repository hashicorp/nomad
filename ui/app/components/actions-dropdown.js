/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';
import { inject as service } from '@ember/service';
import { action } from '@ember/object';

export default class ActionsDropdownComponent extends Component {
  @service nomadActions;
  @service notifications;

  /**
   * @param {HTMLElement} el
   */
  @action openActionsDropdown(el) {
    const dropdownTrigger = el?.getElementsByTagName('button')[0];
    if (dropdownTrigger) {
      dropdownTrigger.click();
    }
  }
}
