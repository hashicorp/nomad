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
