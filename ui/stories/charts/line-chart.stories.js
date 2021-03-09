import hbs from 'htmlbars-inline-precompile';

import EmberObject from '@ember/object';
import { on } from '@ember/object/evented';
import moment from 'moment';

import DelayedArray from '../utils/delayed-array';

export default {
  title: 'Charts|Line Chart',
};

let data1 = [
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

let data2 = [
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

export let Standard = () => {
  return {
    template: hbs`
      <h5 class="title is-5">Line Chart</h5>
      <div class="block" style="height:100px; width: 400px;">
        {{#if this.lineChartData}}
          <LineChart @data={{this.lineChartData}} @xProp="year" @yProp="value">
            <:svg as |c|>
              <c.Area @data={{this.lineChartData}} />
            </:svg>
          </LineChart>
        {{/if}}
      </div>
      <div class="block" style="height:100px; width: 400px;">
        {{#if this.lineChartMild}}
          <LineChart @data={{this.lineChartMild}} @xProp="year" @yProp="value" @chartClass="is-info">
            <:svg as |c|>
              <c.Area @data={{this.lineChartMild}} />
            </:svg>
          </LineChart>
        {{/if}}
      </div>
      `,
    context: {
      lineChartData: DelayedArray.create(data1),
      lineChartMild: DelayedArray.create(data2),
    },
  };
};

export let FluidWidth = () => {
  return {
    template: hbs`
      <h5 class="title is-5">Fluid-width Line Chart</h5>
      <div class="block" style="height:250px;">
        {{#if this.lineChartData}}
          <LineChart @data={{this.lineChartData}} @xProp="year" @yProp="value" @chartClass="is-danger">
            <:svg as |c|>
              <c.Area @data={{this.lineChartData}} />
            </:svg>
          </LineChart>
        {{/if}}
      </div>
      <div class="block" style="height:250px;">
        {{#if this.lineChartMild}}
          <LineChart @data={{this.lineChartMild}} @xProp="year" @yProp="value" @chartClass="is-warning">
            <:svg as |c|>
              <c.Area @data={{this.lineChartMild}} />
            </:svg>
          </LineChart>
        {{/if}}
      </div>
      <p class="annotation">A line chart will assume the width of its container. This includes the dimensions of the axes, which are calculated based on real DOM measurements. This requires a two-pass render: first the axes are placed with their real domains (in order to capture width and height of tick labels), second the axes are adjusted to make sure both the x and y axes are within the height and width bounds of the container.</p>
      `,
    context: {
      lineChartData: DelayedArray.create(data1),
      lineChartMild: DelayedArray.create(data2),
    },
  };
};

export let LiveData = () => {
  return {
    template: hbs`
      <h5 class="title is-5">Live data Line Chart</h5>
      <div class="block" style="height:250px">
        {{#if this.controller.lineChartLive}}
          <LineChart
            @data={{this.controller.lineChartLive}}
            @xProp="ts"
            @yProp="val"
            @timeseries={{true}}
            @chartClass="is-primary"
            @xFormat={{this.controller.secondsFormat}}>
            <:svg as |c|>
              <c.Area @data={{this.controller.lineChartLive}} />
            </:svg>
          </LineChart>
        {{/if}}
      </div>
      `,
    context: {
      controller: EmberObject.extend({
        startTimer: on('init', function() {
          this.lineChartLive = [];

          this.set(
            'timer',
            setInterval(() => {
              this.incrementProperty('timerTicks');

              let ref = this.lineChartLive;
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

        get secondsFormat() {
          return date => moment(date).format('HH:mm:ss');
        },
      }).create(),
    },
  };
};

export let Gaps = () => {
  return {
    template: hbs`
      <h5 class="title is-5">Line Chart data with gaps</h5>
      <div class="block" style="height:250px">
        {{#if this.lineChartGapData}}
          <LineChart @data={{this.lineChartGapData}} @xProp="year" @yProp="value" @chartClass="is-primary">
            <:svg as |c|>
              <c.Area @data={{this.lineChartGapData}} />
            </:svg>
          </LineChart>
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

export let Annotations = () => {
  return {
    template: hbs`
      <h5 class="title is-5">Line Chart data with annotations</h5>
      <div class="block" style="height:250px">
        {{#if (and this.data this.annotations)}}
          <LineChart
            class="with-annotations"
            @timeseries={{true}}
            @xProp="x"
            @yProp="y"
            @data={{this.data}}>
            <:svg as |c|>
              <c.Area @data={{this.data}} @annotationClick={{action (mut this.activeAnnotation)}} />
            </:svg>
            <:after as |c|>
              <c.VAnnotations
                @annotations={{this.annotations}}
                @annotationClick={{action (mut this.activeAnnotation)}}
                @activeAnnotation={{this.activeAnnotation}} />
            </:after>
          </LineChart>
        {{/if}}
      </div>
      <p style="margin:2em 0; padding: 1em; background:#FFEEAC">{{this.activeAnnotation.info}}</p>
      <h5 class="title is-5">Line Chart data with staggered annotations</h5>
      <div class="block" style="height:150px; width:450px">
        {{#if (and this.data this.annotations)}}
          <LineChart
            class="with-annotations"
            @timeseries={{true}}
            @xProp="x"
            @yProp="y"
            @data={{this.data}}>
            <:svg as |c|>
              <c.Area @data={{this.data}} @annotationClick={{action (mut this.activeAnnotation)}} />
            </:svg>
            <:after as |c|>
              <c.VAnnotations
                @annotations={{this.annotations}}
                @annotationClick={{action (mut this.activeAnnotation)}}
                @activeAnnotation={{this.activeAnnotation}} />
            </:after>
          </LineChart>
        {{/if}}
      </div>
    `,
    context: {
      data: DelayedArray.create(
        new Array(180).fill(null).map((_, idx) => ({
          y: Math.sin((idx * 4 * Math.PI) / 180) * 100 + 200,
          x: moment()
            .add(idx, 'd')
            .toDate(),
        }))
      ),
      annotations: [
        {
          x: moment().toDate(),
          type: 'info',
          info: 'Far left',
        },
        {
          x: moment()
            .add(90 / 4, 'd')
            .toDate(),
          type: 'error',
          info: 'This is the max of the sine curve',
        },
        {
          x: moment()
            .add(89, 'd')
            .toDate(),
          type: 'info',
          info: 'This is the end of the first period',
        },
        {
          x: moment()
            .add(96, 'd')
            .toDate(),
          type: 'info',
          info: 'A close annotation for staggering purposes',
        },
        {
          x: moment()
            .add((90 / 4) * 3, 'd')
            .toDate(),
          type: 'error',
          info: 'This is the min of the sine curve',
        },
        {
          x: moment()
            .add(179, 'd')
            .toDate(),
          type: 'info',
          info: 'Far right',
        },
      ],
    },
  };
};

export let StepLine = () => {
  return {
    template: hbs`
      <h5 class="title is-5">Line Chart with a Step Line</h5>
      <div class="block" style="height:250px">
        {{#if this.data}}
          <LineChart
            @xProp="x"
            @yProp="y"
            @data={{this.data}}>
            <:svg as |c|>
              <c.Area @data={{this.data}} @curve="stepAfter" />
            </:svg>
          </LineChart>
          <p>{{this.activeAnnotation.info}}</p>
        {{/if}}
      </div>
    `,
    context: {
      data: DelayedArray.create([
        { x: 1, y: 5 },
        { x: 2, y: 1 },
        { x: 3, y: 2 },
        { x: 4, y: 2 },
        { x: 5, y: 9 },
        { x: 6, y: 3 },
        { x: 7, y: 4 },
        { x: 8, y: 1 },
        { x: 9, y: 5 },
      ]),
    },
  };
};
