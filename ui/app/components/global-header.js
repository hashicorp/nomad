/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@ember/component';
import classic from 'ember-classic-decorator';
import { inject as service } from '@ember/service';
import { attributeBindings } from '@ember-decorators/component';
import { htmlSafe } from '@ember/template';

@classic
@attributeBindings('data-test-global-header')
export default class GlobalHeader extends Component {
  @service config;
  @service system;

  'data-test-global-header' = true;
  onHamburgerClick() {}

  // Show sign-in if:
  // - User can't load agent config (meaning ACLs are enabled but they're not signed in)
  // - User can load agent config in and ACLs are enabled (meaning ACLs are enabled and they're signed in)
  // The excluded case here is if there is both an agent config and ACLs are disabled
  get shouldShowProfileNav() {
    return (
      !this.system.agent?.get('config') ||
      this.system.agent?.get('config.ACL.Enabled') === true
    );
  }

  get labelStyles() {
    return htmlSafe(
      `
        color: ${this.system.agent.get('config')?.UI?.Label?.TextColor};
        background-color: ${
          this.system.agent.get('config')?.UI?.Label?.BackgroundColor
        };
      `
    );
  }
}
