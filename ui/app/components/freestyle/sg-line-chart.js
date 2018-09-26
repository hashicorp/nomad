import Component from '@ember/component';
import { computed } from '@ember/object';
import d3TimeFormat from 'd3-time-format';

export default Component.extend({
  timerTicks: 0,

  startTimer: function() {
    this.set(
      'timer',
      setInterval(() => {
        this.incrementProperty('timerTicks');

        const ref = this.get('lineChartLive');
        ref.addObject({ ts: Date.now(), val: Math.random() * 30 + 20 });
        if (ref.length > 60) {
          ref.splice(0, ref.length - 60);
        }
      }, 500)
    );
  }.on('init'),

  willDestroy() {
    clearInterval(this.get('timer'));
  },

  lineChartData: computed(() => {
    return [
      { year: 2010, value: 10 },
      { year: 2011, value: 10 },
      { year: 2012, value: 20 },
      { year: 2013, value: 30 },
      { year: 2014, value: 50 },
      { year: 2015, value: 80 },
      { year: 2016, value: 130 },
      { year: 2017, value: 210 },
      { year: 2018, value: 340 },
    ];
  }),

  lineChartMild: computed(() => {
    return [
      { year: 2010, value: 100 },
      { year: 2011, value: 90 },
      { year: 2012, value: 120 },
      { year: 2013, value: 130 },
      { year: 2014, value: 115 },
      { year: 2015, value: 105 },
      { year: 2016, value: 90 },
      { year: 2017, value: 85 },
      { year: 2018, value: 90 },
    ];
  }),

  lineChartGapData: computed(() => {
    return [
      { year: 2010, value: 10 },
      { year: 2011, value: 10 },
      { year: 2012, value: null },
      { year: 2013, value: 30 },
      { year: 2014, value: 50 },
      { year: 2015, value: 80 },
      { year: 2016, value: null },
      { year: 2017, value: 210 },
      { year: 2018, value: 340 },
    ];
  }),

  lineChartLive: computed(() => {
    return [];
  }),

  secondsFormat() {
    return d3TimeFormat.timeFormat('%H:%M:%S');
  },
});
