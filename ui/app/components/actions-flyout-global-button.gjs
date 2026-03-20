/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';
import { service } from '@ember/service';
import { on } from '@ember/modifier';
import { HdsButton } from '@hashicorp/design-system-components/components';
import keyboardShortcut from 'nomad-ui/modifiers/keyboard-shortcut';

export default class ActionsFlyoutGlobalButton extends Component {
  @service nomadActions;

  shortcutPattern = ['a', 'c'];

  get runningActionsCount() {
    return this.nomadActions.runningActions.length;
  }

  get shouldShow() {
    return (
      this.nomadActions.actionsQueue.length > 0 &&
      !this.nomadActions.flyoutActive
    );
  }

  get buttonText() {
    const count = this.runningActionsCount;
    if (!count) return 'Actions';

    const label = count === 1 ? 'Action' : 'Actions';
    return `${count} ${label} Running`;
  }

  get buttonIcon() {
    return this.runningActionsCount ? 'loading' : 'chevron-right';
  }

  <template>
    {{#if this.shouldShow}}
      <div
        class="actions-flyout-button"
        {{keyboardShortcut
          menuLevel=true
          pattern=this.shortcutPattern
          action=this.nomadActions.openFlyout
        }}
        ...attributes
      >
        <HdsButton
          {{on "click" this.nomadActions.openFlyout}}
          disabled={{this.nomadActions.flyoutActive}}
          @text={{this.buttonText}}
          @icon={{this.buttonIcon}}
          @iconPosition="trailing"
          @color="secondary"
        />
      </div>
    {{/if}}
  </template>
}
