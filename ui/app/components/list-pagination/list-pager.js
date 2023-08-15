/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@ember/component';
import { tagName } from '@ember-decorators/component';
import classic from 'ember-classic-decorator';
import { action } from '@ember/object';
import { inject as service } from '@ember/service';
import KeyboardShortcutModifier from 'nomad-ui/modifiers/keyboard-shortcut';

@classic
@tagName('')
export default class ListPager extends Component {
  @service router;

  // Even though we don't currently use "first" / "last" pagination in the app,
  // the option is there at a component level, so let's make sure that we
  // only append keyNav to the "next" and "prev" links.
  // We use this to make the modifier conditional, per https://v5.chriskrycho.com/journal/conditional-modifiers-and-helpers-in-emberjs/
  get includeKeyboardNav() {
    return this.label === 'Next page' || this.label === 'Previous page'
      ? KeyboardShortcutModifier
      : null;
  }

  @action
  gotoRoute() {
    this.router.transitionTo(this.router.currentRouteName, {
      queryParams: { page: this.page },
    });
  }
}
