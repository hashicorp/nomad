/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';
import { service } from '@ember/service';
import { debounce } from '@ember/runloop';
import { on } from '@ember/modifier';
import { HdsFormTextInputBase } from '@hashicorp/design-system-components/components';
import keyboardShortcut from 'nomad-ui/modifiers/keyboard-shortcut';

const DEBOUNCE_MS = 500;

export default class JobSearchBox extends Component {
  @service keyboard;

  shortcutPattern = ['Shift+F'];

  updateSearchText = (event) => {
    debounce(this, this.sendUpdate, event.target.value, DEBOUNCE_MS);
  };

  sendUpdate = (value) => {
    this.args.onSearchTextChange(value);
  };

  get textInputComponent() {
    return this.args.s?.TextInput || HdsFormTextInputBase;
  }

  focus = (element) => {
    element.focus();
    // Because the element is an input,
    // and the "hide hints" part of our keynav implementation is on keyUp,
    // but the focus action happens on keyDown,
    // and the keynav explicitly ignores key input while focused in a text input,
    // we need to manually hide the hints here.
    this.keyboard.displayHints = false;
  };

  <template>
    <this.textInputComponent
      @type="search"
      @value={{@searchText}}
      aria-label="Job Search"
      placeholder="Name contains myJob"
      @icon="search"
      @width="300px"
      {{on "input" this.updateSearchText}}
      {{keyboardShortcut
        label="Search Jobs"
        pattern=this.shortcutPattern
        action=this.focus
      }}
      data-test-jobs-search
    />
  </template>
}
