/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';
import { hash } from '@ember/helper';
import { LinkTo } from '@ember/routing';
import { service } from '@ember/service';
import KeyboardShortcutModifier from 'nomad-ui/modifiers/keyboard-shortcut';

export default class ListPager extends Component {
  @service router;

  // Even though we don't currently use "first" / "last" pagination in the app,
  // the option is there at a component level, so let's make sure that we
  // only append keyNav to the "next" and "prev" links.
  // We use this to make the modifier conditional, per https://v5.chriskrycho.com/journal/conditional-modifiers-and-helpers-in-emberjs/
  get includeKeyboardNav() {
    return this.args.label === 'Next page' ||
      this.args.label === 'Previous page'
      ? KeyboardShortcutModifier
      : null;
  }

  get keyboardLabel() {
    return this.args.label === 'Next page' ? 'Next Page' : 'Previous Page';
  }

  get keyboardPattern() {
    return this.args.label === 'Next page' ? [']', ']'] : ['[', '['];
  }

  gotoRoute = () => {
    this.router.transitionTo({
      queryParams: { page: this.args.page },
    });
  };

  <template>
    {{#if @visible}}
      <LinkTo
        @query={{hash currentPage=@page}}
        class={{@class}}
        data-test-pager={{@test}}
        aria-label={{@label}}
        ...attributes
        {{(modifier
          this.includeKeyboardNav
          label=this.keyboardLabel
          action=this.gotoRoute
          pattern=this.keyboardPattern
        )}}
      >
        {{yield}}
      </LinkTo>
    {{/if}}
  </template>
}
