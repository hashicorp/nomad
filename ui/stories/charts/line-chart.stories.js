import hbs from 'htmlbars-inline-precompile';

import EmberObject from '@ember/object';
import { on } from '@ember/object/evented';
import moment from 'moment';

import DelayedArray from '../delayed-array';

export default {
  title: 'Charts|Line Chart',
};

const data1 = [
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

const data2 = [
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

export const Standard = () => {
  return {
    template: hbs`
      <h5 class="title is-5">Line Chart</h5>
      <div class="block" style="height:100px; width: 400px;">
        {{#if lineChartData}}
          <LineChart @data={{lineChartData}} @xProp="year" @yProp="value" @chartClass="is-primary" />
        {{/if}}
      </div>
      <div class="block" style="height:100px; width: 400px;">
        {{#if lineChartMild}}
          <LineChart @data={{lineChartMild}} @xProp="year" @yProp="value" @chartClass="is-info" />
        {{/if}}
      </div>
      `,
    context: {
      lineChartData: DelayedArray.create(data1),
      lineChartMild: DelayedArray.create(data2),
    },
  };
};

export const FluidWidth = () => {
  return {
    template: hbs`
      <h5 class="title is-5">Fluid-width Line Chart</h5>
      <div class="block" style="height:250px;">
        {{#if lineChartData}}
          <LineChart @data={{lineChartData}} @xProp="year" @yProp="value" @chartClass="is-danger" />
        {{/if}}
      </div>
      <div class="block" style="height:250px;">
        {{#if lineChartMild}}
          <LineChart @data={{lineChartMild}} @xProp="year" @yProp="value" @chartClass="is-warning" />
        {{/if}}
      </div>
      <p class='annotation'>A line chart will assume the width of its container. This includes the dimensions of the axes, which are calculated based on real DOM measurements. This requires a two-pass render: first the axes are placed with their real domains (in order to capture width and height of tick labels), second the axes are adjusted to make sure both the x and y axes are within the height and width bounds of the container.</p>
      `,
    context: {
      lineChartData: DelayedArray.create(data1),
      lineChartMild: DelayedArray.create(data2),
    },
  };
};

export const LiveData = () => {
  return {
    template: hbs`
      <h5 class="title is-5">Live data Line Chart</h5>
      <div class="block" style="height:250px">
        {{#if controller.lineChartLive}}
          <LineChart @data={{controller.lineChartLive}} @xProp="ts" @yProp="val" @timeseries={{true}} @chartClass="is-primary" @xFormat={{controller.secondsFormat}} />
        {{/if}}
      </div>
      `,
    context: {
      controller: EmberObject.extend({
        startTimer: on('init', function() {
          this.set(
            'timer',
            setInterval(() => {
              this.incrementProperty('timerTicks');

              const ref = this.lineChartLive;
              ref.addObject({ ts: Date.now(), val: Math.random() * 30 + 20 });
              if (ref.length > 60) {
                ref.splice(0, ref.length - 60);
              }
            }, 500)
          );
        }),

        willDestroy() {
          clearInterval(this.timer);
        },

        lineChartLive: [],

        secondsFormat() {
          return date => moment(date).format('HH:mm:ss');
        },
      }).create(),
    },
  };
};

export const Gaps = () => {
  return {
    template: hbs`
      <h5 class="title is-5">Line Chart data with gaps</h5>
      <div class="block" style="height:250px">
        {{#if lineChartGapData}}
          <LineChart @data={{lineChartGapData}} @xProp="year" @yProp="value" @chartClass="is-primary" />
        {{/if}}
      </div>
      `,
    context: {
      lineChartGapData: DelayedArray.create([
        { year: 2010, value: 10 },
        { year: 2011, value: 10 },
        { year: 2012, value: null },
        { year: 2013, value: 30 },
        { year: 2014, value: 50 },
        { year: 2015, value: 80 },
        { year: 2016, value: null },
        { year: 2017, value: 210 },
        { year: 2018, value: 340 },
      ]),
    },
  };
};
