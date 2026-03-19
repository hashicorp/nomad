/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';
import { LinkTo } from '@ember/routing';
import { HdsTooltipButton } from '@hashicorp/design-system-components/components';
import hdsTooltip from '@hashicorp/design-system-components/modifiers/hds-tooltip';

export default class ConditionalLinkToComponent extends Component {
  get query() {
    return this.args.query || {};
  }

  get tooltipText() {
    return this.args.tooltip?.text || '';
  }

  <template>
    {{#if @condition}}
      {{#if @tooltip}}
        <LinkTo
          @route={{@route}}
          @model={{@model}}
          @query={{this.query}}
          class={{@class}}
          aria-label={{@label}}
          {{hdsTooltip @tooltip.text options=@tooltip.extraTippyOptions}}
        >
          {{yield}}
        </LinkTo>
      {{else}}
        <LinkTo
          @route={{@route}}
          @model={{@model}}
          @query={{this.query}}
          class={{@class}}
          aria-label={{@label}}
        >
          {{yield}}
        </LinkTo>
      {{/if}}
    {{else}}
      {{#if @tooltip}}
        <HdsTooltipButton
          @text={{this.tooltipText}}
          @extraTippyOptions={{@tooltip.extraTippyOptions}}
        >
          <span class={{@class}}>
            {{yield}}
          </span>
        </HdsTooltipButton>
      {{else}}
        <span class={{@class}}>
          {{yield}}
        </span>
      {{/if}}
    {{/if}}
  </template>
}
