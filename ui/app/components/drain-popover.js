/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@ember/component';
import { computed } from '@ember/object';
import { equal } from '@ember/object/computed';
import { computed as overridable } from 'ember-overridable-computed';
import { task } from 'ember-concurrency';
import Duration from 'duration-js';
import { tagName } from '@ember-decorators/component';
import classic from 'ember-classic-decorator';
import localStorageProperty from 'nomad-ui/utils/properties/local-storage';

@classic
@tagName('')
export default class DrainPopover extends Component {
  client = null;
  isDisabled = false;

  onError() {}
  onDrain() {}

  parseError = '';

  deadlineEnabled = false;
  forceDrain = false;
  drainSystemJobs = true;

  @localStorageProperty('nomadDrainOptions', {}) drainOptions;

  didReceiveAttrs() {
    super.didReceiveAttrs();
    // Load drain config values from local storage if availabe.
    [
      'deadlineEnabled',
      'customDuration',
      'forceDrain',
      'drainSystemJobs',
      'selectedDurationQuickOption',
    ].forEach((k) => {
      if (k in this.drainOptions) {
        this[k] = this.drainOptions[k];
      }
    });
  }

  @overridable(function () {
    return this.durationQuickOptions[0];
  })
  selectedDurationQuickOption;

  @equal('selectedDurationQuickOption.value', 'custom') durationIsCustom;
  customDuration = '';

  @computed
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

  @computed(
    'deadlineEnabled',
    'durationIsCustom',
    'customDuration',
    'selectedDurationQuickOption.value'
  )
  get deadline() {
    if (!this.deadlineEnabled) return 0;
    if (this.durationIsCustom) return this.customDuration;
    return this.selectedDurationQuickOption.value;
  }

  @task(function* (close) {
    if (!this.client) return;
    const isUpdating = this.client.isDraining;

    let deadline;
    try {
      deadline = new Duration(this.deadline).nanoseconds();
    } catch (err) {
      this.set('parseError', err.message);
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
        yield this.client.forceDrain(spec);
      } else {
        yield this.client.drain(spec);
      }
      this.onDrain(isUpdating);
    } catch (err) {
      this.onError(err);
    }
  })
  drain;

  preventDefault(e) {
    e.preventDefault();
  }
}
