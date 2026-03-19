/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';
import { tracked } from '@glimmer/tracking';
import { fn } from '@ember/helper';
import { on } from '@ember/modifier';
import { task } from 'ember-concurrency';
import { HdsIcon } from '@hashicorp/design-system-components/components';
import Duration from 'duration-js';
import PowerSelect from 'ember-power-select/components/power-select';
import PopoverMenu from 'nomad-ui/components/popover-menu';
import Toggle from 'nomad-ui/components/toggle';
import localStorageProperty from 'nomad-ui/utils/properties/local-storage';

const noOp = () => {};

export default class DrainPopover extends Component {
  @tracked parseError = '';
  @tracked deadlineEnabled = false;
  @tracked forceDrain = false;
  @tracked drainSystemJobs = true;
  @tracked customDuration = '';
  @tracked selectedDurationQuickOption;

  @localStorageProperty('nomadDrainOptions', {}) drainOptions;

  constructor() {
    super(...arguments);

    this.selectedDurationQuickOption = this.durationQuickOptions[0];

    [
      'deadlineEnabled',
      'customDuration',
      'forceDrain',
      'drainSystemJobs',
      'selectedDurationQuickOption',
    ].forEach((key) => {
      if (key in this.drainOptions) {
        this[key] = this.drainOptions[key];
      }
    });
  }

  get client() {
    return this.args.client;
  }

  get isDisabled() {
    return this.args.isDisabled ?? false;
  }

  get onError() {
    return this.args.onError ?? noOp;
  }

  get onDrain() {
    return this.args.onDrain ?? noOp;
  }

  get durationQuickOptions() {
    return [
      { label: '1 Hour', value: '1h' },
      { label: '4 Hours', value: '4h' },
      { label: '8 Hours', value: '8h' },
      { label: '12 Hours', value: '12h' },
      { label: '1 Day', value: '1d' },
      { label: 'Custom', value: 'custom' },
    ];
  }

  get durationIsCustom() {
    return this.selectedDurationQuickOption?.value === 'custom';
  }

  get deadline() {
    if (!this.deadlineEnabled) return 0;
    if (this.durationIsCustom) return this.customDuration;
    return this.selectedDurationQuickOption.value;
  }

  get popoverLabel() {
    return this.client?.isDraining ? 'Update Drain' : 'Drain';
  }

  get popoverTooltip() {
    return this.isDisabled ? 'Not allowed to drain clients' : undefined;
  }

  get triggerClass() {
    return [
      'is-small',
      this.drain.isRunning ? 'is-loading' : '',
      this.isDisabled ? 'tooltip is-right-aligned' : '',
    ]
      .filter(Boolean)
      .join(' ');
  }

  get customDurationInputValue() {
    return this.customDuration === 0 ? '' : this.customDuration;
  }

  drain = task({ drop: true }, async (close) => {
    if (!this.client) return;
    const isUpdating = this.client.isDraining;

    let deadline;
    try {
      deadline = new Duration(this.deadline).nanoseconds();
    } catch (err) {
      this.parseError = err.message;
      return;
    }

    const spec = {
      Deadline: deadline,
      IgnoreSystemJobs: !this.drainSystemJobs,
    };

    this.drainOptions = {
      deadlineEnabled: this.deadlineEnabled,
      customDuration: this.deadline,
      selectedDurationQuickOption: this.selectedDurationQuickOption,
      drainSystemJobs: this.drainSystemJobs,
      forceDrain: this.forceDrain,
    };

    close();

    try {
      if (this.forceDrain) {
        await this.client.forceDrain(spec);
      } else {
        await this.client.drain(spec);
      }
      this.onDrain(isUpdating);
    } catch (err) {
      this.onError(err);
    }
  });

  onSubmit = (event, close) => {
    event.preventDefault();
    this.drain.perform(close);
  };

  onClickDrain = (close) => {
    this.drain.perform(close);
  };

  updateDeadlineEnabled = ({ target: { checked } }) => {
    this.deadlineEnabled = checked;
  };

  updateForceDrain = ({ target: { checked } }) => {
    this.forceDrain = checked;
  };

  updateDrainSystemJobs = ({ target: { checked } }) => {
    this.drainSystemJobs = checked;
  };

  updateSelectedDuration = (option) => {
    this.selectedDurationQuickOption = option;
  };

  updateCustomDuration = ({ target: { value } }) => {
    this.parseError = '';
    this.customDuration = value;
  };

  <template>
    {{! template-lint-disable require-input-label }}
    <PopoverMenu
      data-test-drain-popover
      @isDisabled={{this.isDisabled}}
      @label={{this.popoverLabel}}
      @tooltip={{this.popoverTooltip}}
      @triggerClass={{this.triggerClass}}
      as |m|
    >
      <form
        data-test-drain-popover-form
        {{on "submit" (fn this.onSubmit m.actions.close)}}
        class="form is-small"
      >
        <h4 class="group-heading">Drain Options</h4>
        <div class="field">
          <label class="label is-interactive">
            <Toggle
              data-test-drain-deadline-toggle
              @isActive={{this.deadlineEnabled}}
              @onToggle={{this.updateDeadlineEnabled}}
            >
              Deadline
            </Toggle>
            <span
              class="tooltip multiline"
              aria-label="The amount of time a drain must complete within."
            >
              <HdsIcon @name="info" @color="faint" @isInline={{true}} />
            </span>
          </label>
        </div>
        {{#if this.deadlineEnabled}}
          <div
            class="field is-sub-field"
            data-test-drain-deadline-option-select-parent
          >
            <PowerSelect
              data-test-drain-deadline-option-select
              @ariaLabel="label-drain-deadline-option-select"
              @ariaLabelledBy="label-drain-deadline-option-select"
              @tagName="div"
              @options={{this.durationQuickOptions}}
              @selected={{this.selectedDurationQuickOption}}
              @onChange={{this.updateSelectedDuration}}
              as |opt|
            >
              {{opt.label}}
            </PowerSelect>
          </div>
          {{#if this.durationIsCustom}}
            <div class="field is-sub-field">
              <label class="label">Deadline</label>
              <input
                data-test-drain-custom-deadline
                type="text"
                class="input {{if this.parseError 'is-danger'}}"
                placeholder="1h30m"
                value={{this.customDurationInputValue}}
                {{on "input" this.updateCustomDuration}}
              />
              {{#if this.parseError}}
                <em class="help is-danger">{{this.parseError}}</em>
              {{/if}}
            </div>
          {{/if}}
        {{/if}}
        <div class="field">
          <label class="label is-interactive">
            <Toggle
              data-test-force-drain-toggle
              @isActive={{this.forceDrain}}
              @onToggle={{this.updateForceDrain}}
            >
              Force Drain
            </Toggle>
            <span
              class="tooltip multiline"
              aria-label="Immediately remove allocations from the client."
            >
              <HdsIcon @name="info" @color="faint" @isInline={{true}} />
            </span>
          </label>
        </div>
        <div class="field">
          <label class="label is-interactive">
            <Toggle
              data-test-system-jobs-toggle
              @isActive={{this.drainSystemJobs}}
              @onToggle={{this.updateDrainSystemJobs}}
            >
              Drain System Jobs
            </Toggle>
            <span
              class="tooltip multiline"
              aria-label="Stop allocations for system jobs."
            >
              <HdsIcon @name="info" @color="faint" @isInline={{true}} />
            </span>
          </label>
        </div>
        <div class="popover-actions">
          <button
            data-test-drain-submit
            type="button"
            class="popover-action is-primary"
            {{on "click" (fn this.onClickDrain m.actions.close)}}
          >
            Drain
          </button>
          <button
            data-test-drain-cancel
            type="button"
            class="popover-action"
            {{on "click" m.actions.close}}
          >Cancel</button>
        </div>
      </form>
    </PopoverMenu>
  </template>
}
