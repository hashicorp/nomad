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
      }, 1000)
    );
  }),

  willDestroy() {
    clearInterval(this.timer);
  },

  denominator: computed('timerTicks', function() {
    return Math.round(Math.random() * 1000);
  }),

  percentage: computed('timerTicks', function() {
    return Math.round(Math.random() * 100) / 100;
  }),

  numerator: computed('denominator', 'percentage', function() {
    return Math.round(this.denominator * this.percentage * 100) / 100;
  }),

  liveDetails: computed('denominator', 'numerator', 'percentage', function() {
    return this.getProperties('denominator', 'numerator', 'percentage');
  }),
});
