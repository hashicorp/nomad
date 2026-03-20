/**
 * Copyright IBM Corp. 2015, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';
import { service } from '@ember/service';
import { LinkTo } from '@ember/routing';
import KeyboardShortcutModifier from 'nomad-ui/modifiers/keyboard-shortcut';

export default class BreadcrumbsTemplate extends Component {
  @service router;

  shortcutPattern = ['u'];

  /**
   * The route name extracted from the crumb args array.
   * @returns {string}
   */
  get route() {
    return this.args.crumb?.args?.[0];
  }

  /**
   * The dynamic segments (models) for the route, extracted from the crumb args array.
   * @returns {Array}
   */
  get models() {
    return this.args.crumb?.args?.slice(1) ?? [];
  }

  get isOneCrumbUp() {
    return this.args.isOneCrumbUp?.() ?? false;
  }

  traverseUpALevel = () => {
    this.router.transitionTo(this.route, ...this.models);
  };

  <template>
    {{#if this.isOneCrumbUp}}
      <li
        data-test-breadcrumb-default
        ...attributes
        {{KeyboardShortcutModifier
          label="Go up a level"
          pattern=this.shortcutPattern
          menuLevel=true
          action=this.traverseUpALevel
          exclusive=true
        }}
      >
        <LinkTo
          @route={{this.route}}
          @models={{this.models}}
          data-test-breadcrumb={{this.route}}
        >
          {{#if @crumb.title}}
            <dl>
              <dt>
                {{@crumb.title}}
              </dt>
              <dd>
                {{@crumb.label}}
              </dd>
            </dl>
          {{else}}
            {{@crumb.label}}
          {{/if}}
        </LinkTo>
      </li>
    {{else}}
      <li data-test-breadcrumb-default ...attributes>
        <LinkTo
          @route={{this.route}}
          @models={{this.models}}
          data-test-breadcrumb={{this.route}}
        >
          {{#if @crumb.title}}
            <dl>
              <dt>
                {{@crumb.title}}
              </dt>
              <dd>
                {{@crumb.label}}
              </dd>
            </dl>
          {{else}}
            {{@crumb.label}}
          {{/if}}
        </LinkTo>
      </li>
    {{/if}}
  </template>
}
