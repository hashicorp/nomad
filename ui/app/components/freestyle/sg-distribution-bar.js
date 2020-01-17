import Component from '@ember/component';
import { computed } from '@ember/object';
import { on } from '@ember/object/evented';

export default Component.extend({
  timerTicks: 0,

  startTimer: on('init', function() {
    this.set(
      'timer',
      setInterval(() => {
        this.incrementProperty('timerTicks');
      }, 500)
    );
  }),

  willDestroy() {
    clearInterval(this.timer);
  },

  distributionBarDatum: computed(() => {
    return [{ label: 'one', value: 10 }];
  }),

  distributionBarData: computed(() => {
    return [
      { label: 'one', value: 10 },
      { label: 'two', value: 20 },
      { label: 'three', value: 30 },
    ];
  }),

  distributionBarDataWithClasses: computed(() => {
    return [
      { label: 'Queued', value: 10, className: 'queued' },
      { label: 'Complete', value: 20, className: 'complete' },
      { label: 'Failed', value: 30, className: 'failed' },
    ];
  }),

  distributionBarDataRotating: computed('timerTicks', () => {
    return [
      { label: 'one', value: Math.round(Math.random() * 50) },
      { label: 'two', value: Math.round(Math.random() * 50) },
      { label: 'three', value: Math.round(Math.random() * 50) },
    ];
  }),
});
