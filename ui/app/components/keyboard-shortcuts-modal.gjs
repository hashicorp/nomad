/**
 * Copyright IBM Corp. 2015, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';
import { array, fn } from '@ember/helper';
import { on } from '@ember/modifier';
import { service } from '@ember/service';
import { didInsert, willDestroy } from '@ember/render-modifiers';
import onClickOutside from 'ember-click-outside/modifiers/on-click-outside';
import { and, not, or } from 'ember-truth-helpers';
import {
  HdsFormToggleField,
  HdsIcon,
} from '@hashicorp/design-system-components/components';
import cleanKeycommand from 'nomad-ui/helpers/clean-keycommand';
import keyboardCommands from 'nomad-ui/helpers/keyboard-commands';
import autofocus from 'nomad-ui/modifiers/autofocus';
import Tether from 'tether';

export default class KeyboardShortcutsModal extends Component {
  @service keyboard;
  @service config;

  blurHandler = () => {
    if (this.isDestroying || this.isDestroyed) {
      return;
    }

    const keyboard = this.keyboard;
    if (!keyboard || keyboard.isDestroying || keyboard.isDestroyed) {
      return;
    }

    keyboard.displayHints = false;
  };

  constructor() {
    super(...arguments);
    window.addEventListener('blur', this.blurHandler);
  }

  willDestroy() {
    super.willDestroy(...arguments);
    window.removeEventListener('blur', this.blurHandler);
  }

  escapeCommand = {
    label: 'Hide Keyboard Shortcuts',
    pattern: ['Escape'],
    action: () => {
      this.keyboard.shortcutsVisible = false;
    },
  };

  get commands() {
    return this.keyboard.keyCommands.reduce((memo, command) => {
      if (
        command.label &&
        command.action &&
        !memo.find((existing) => existing.label === command.label)
      ) {
        memo.push(command);
      }
      return memo;
    }, []);
  }

  get hints() {
    if (!this.keyboard.displayHints) {
      return [];
    }

    const elementBoundKeyCommands = this.keyboard.keyCommands.filter(
      (command) => command.element,
    );

    return elementBoundKeyCommands.map((command) => {
      const pair = this.keyboard.keyCommands.find(
        (candidate) =>
          JSON.stringify(candidate.defaultPattern) ===
          JSON.stringify(command.pattern),
      );

      if (!pair) {
        return command;
      }

      return {
        ...command,
        pattern: pair.pattern,
      };
    });
  }

  tetherToElement = (element, hint, self) => {
    if (!this.config.isTest) {
      hint.binder = new Tether({
        element: self,
        target: element,
        attachment: 'top left',
        targetAttachment: 'top left',
        targetModifier: 'visible',
      });
    }
  };

  untetherFromElement = (hint) => {
    if (!this.config.isTest) {
      hint.binder.destroy();
    }
  };

  closeShortcuts = () => {
    this.keyboard.shortcutsVisible = false;
  };

  toggleListener = () => {
    this.keyboard.enabled = !this.keyboard.enabled;
  };

  <template>
    {{#if this.keyboard.shortcutsVisible}}
      {{keyboardCommands (array this.escapeCommand)}}
      <section
        class="keyboard-shortcuts"
        {{onClickOutside this.closeShortcuts}}
      >
        <header>
          <button
            {{autofocus}}
            class="button is-borderless dismiss"
            type="button"
            {{on "click" this.closeShortcuts}}
            aria-label="Dismiss"
          >
            <HdsIcon @name="x" />
          </button>
          <h2>Keyboard Shortcuts</h2>
          <p>Click a key pattern to re-bind it to a shortcut of your choosing.</p>
        </header>
        <ul class="commands-list">
          {{#each this.commands as |command|}}
            <li data-test-command-label={{command.label}}>
              <strong>{{command.label}}</strong>
              <span class="keys">
                {{#if command.recording}}
                  <span class="recording">Recording; ESC to cancel.</span>
                {{else}}
                  {{#if command.custom}}
                    <button
                      type="button"
                      class="reset-to-default"
                      {{on
                        "click"
                        (fn this.keyboard.resetCommandToDefault command)
                      }}
                    >reset to default</button>
                  {{/if}}
                {{/if}}

                <button
                  data-test-rebinder
                  disabled={{or (not command.rebindable) command.recording}}
                  type="button"
                  {{on "click" (fn this.keyboard.rebindCommand command)}}
                >
                  {{#each command.pattern as |key|}}
                    <span>{{cleanKeycommand key}}</span>
                  {{/each}}
                </button>
              </span>
            </li>
          {{/each}}
        </ul>
        <footer>
          <HdsFormToggleField
            {{on "change" this.toggleListener}}
            data-test-enable-shortcuts-toggle
            class={{if this.keyboard.enabled "is-active"}}
            checked={{this.keyboard.enabled}}
            as |F|
          >
            <F.Label>Keyboard shortcuts
              {{#if
                this.keyboard.enabled
              }}enabled{{else}}disabled{{/if}}</F.Label>
          </HdsFormToggleField>
        </footer>
      </section>
    {{/if}}

    {{#if (and this.keyboard.enabled this.keyboard.displayHints)}}
      {{#each this.hints as |hint|}}
        <span
          {{didInsert (fn this.tetherToElement hint.element hint)}}
          {{willDestroy (fn this.untetherFromElement hint)}}
          data-test-keyboard-hint
          data-shortcut={{hint.pattern}}
          class={{if hint.menuLevel "menu-level"}}
          aria-hidden="true"
        >
          {{#each hint.pattern as |key|}}
            <span>{{key}}</span>
          {{/each}}
        </span>
      {{/each}}
    {{/if}}
  </template>
}
