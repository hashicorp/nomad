import Component from '@ember/component';
import { computed } from '@ember/object';
import { equal } from '@ember/object/computed';
import { computed as overridable } from 'ember-overridable-computed';
import { task } from 'ember-concurrency';
import Duration from 'duration-js';

export default Component.extend({
  tagName: '',

  client: null,
  isDisabled: false,

  onError() {},
  onDrain() {},

  parseError: '',

  deadlineEnabled: false,
  forceDrain: false,
  drainSystemJobs: true,

  selectedDurationQuickOption: overridable(function() {
    return this.durationQuickOptions[0];
  }),

  durationIsCustom: equal('selectedDurationQuickOption.value', 'custom'),
  customDuration: '',

  durationQuickOptions: computed(() => [
    { label: '1 Hour', value: '1h' },
    { label: '4 Hours', value: '4h' },
    { label: '8 Hours', value: '8h' },
    { label: '12 Hours', value: '12h' },
    { label: '1 Day', value: '1d' },
    { label: 'Custom', value: 'custom' },
  ]),

  deadline: computed(
    'deadlineEnabled',
    'durationIsCustom',
    'customDuration',
    'selectedDurationQuickOption.value',
    function() {
      if (!this.deadlineEnabled) return 0;
      if (this.durationIsCustom) return this.customDuration;
      return this.selectedDurationQuickOption.value;
    }
  ),

  drain: task(function*(close) {
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
  }),

  preventDefault(e) {
    e.preventDefault();
  },
});
