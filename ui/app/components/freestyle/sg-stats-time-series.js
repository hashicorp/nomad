import Component from '@ember/component';
import { computed } from '@ember/object';
import d3TimeFormat from 'd3-time-format';
import moment from 'moment';

export default Component.extend({
  timerTicks: 0,

  startTimer: function() {
    this.set(
      'timer',
      setInterval(() => {
        const metricsHigh = this.get('metricsHigh');
        const prev = metricsHigh.length ? metricsHigh[metricsHigh.length - 1].value : 0.9;
        this.appendTSValue(
          metricsHigh,
          Math.min(Math.max(prev + Math.random() * 0.05 - 0.025, 0.5), 1)
        );

        const metricsLow = this.get('metricsLow');
        const prev2 = metricsLow.length ? metricsLow[metricsLow.length - 1].value : 0.1;
        this.appendTSValue(
          metricsLow,
          Math.min(Math.max(prev2 + Math.random() * 0.05 - 0.025, 0), 0.5)
        );
      }, 1000)
    );
  }.on('init'),

  appendTSValue(array, value, maxLength = 300) {
    array.addObject({
      timestamp: Date.now(),
      value,
    });

    if (array.length > maxLength) {
      array.splice(0, array.length - maxLength);
    }
  },

  willDestroy() {
    clearInterval(this.get('timer'));
  },

  metricsHigh: computed(() => {
    return [];
  }),

  metricsLow: computed(() => {
    return [];
  }),

  staticMetrics: computed(() => {
    const ts = offset =>
      moment()
        .subtract(offset, 'm')
        .toDate();
    return [
      { timestamp: ts(20), value: 0.5 },
      { timestamp: ts(18), value: 0.5 },
      { timestamp: ts(16), value: 0.4 },
      { timestamp: ts(14), value: 0.3 },
      { timestamp: ts(12), value: 0.9 },
      { timestamp: ts(10), value: 0.3 },
      { timestamp: ts(8), value: 0.3 },
      { timestamp: ts(6), value: 0.4 },
      { timestamp: ts(4), value: 0.5 },
      { timestamp: ts(2), value: 0.6 },
      { timestamp: ts(0), value: 0.6 },
    ];
  }),

  secondsFormat() {
    return d3TimeFormat.timeFormat('%H:%M:%S');
  },
});
