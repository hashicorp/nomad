/**
 * Copyright IBM Corp. 2015, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';
import { service } from '@ember/service';
import { on } from '@ember/modifier';
import keyboardShortcut from 'nomad-ui/modifiers/keyboard-shortcut';
import {
  HdsButton,
  HdsDropdown,
} from '@hashicorp/design-system-components/components';

export default class ProfileNavbarItem extends Component {
  @service token;
  @service router;
  @service store;

  profileShortcut = ['g', 'p'];

  get profileName() {
    return this.token.selfToken?.name || 'Profile';
  }

  signOut = () => {
    this.token.setProperties({
      secret: undefined,
    });

    // Clear out all data to ensure only data the anonymous token is privileged to see is shown.
    this.store.unloadAll();
    this.token.reset();
    this.router.transitionTo('jobs.index');
  };

  <template>
    {{#if this.token.selfToken}}
      <HdsDropdown
        @color="secondary"
        class="profile-dropdown"
        {{keyboardShortcut menuLevel=true pattern=this.profileShortcut}}
        as |dd|
      >
        <dd.ToggleButton
          @color="secondary"
          @icon="user-circle"
          @text={{this.profileName}}
          @size="small"
          data-test-header-profile-dropdown
        />
        <dd.Description @text="Signed In" />
        <dd.Separator />
        <dd.Interactive
          @route="settings.tokens"
          @text="Profile"
          data-test-profile-dropdown-profile-link
        />
        <dd.Interactive
          {{on "click" this.signOut}}
          @text="Sign Out"
          @color="critical"
          data-test-profile-dropdown-sign-out-link
        />
      </HdsDropdown>
    {{else}}
      <span
        class="profile-link"
        {{keyboardShortcut menuLevel=true pattern=this.profileShortcut}}
      >
        <HdsButton
          data-test-header-signin-link
          @route="settings.tokens"
          @text="Profile and Sign In"
          @icon="user-circle"
          @isIconOnly={{true}}
          @color="secondary"
          @size="medium"
        />
      </span>
    {{/if}}

    {{yield}}
  </template>
}
