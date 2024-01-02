/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import hbs from 'htmlbars-inline-precompile';

import EmberObject, { computed } from '@ember/object';
import { on } from '@ember/object/evented';

export default {
  title: 'Charts/Progress Bar',
};

export let Standard = () => {
  return {
    template: hbs`
      <h5 class="title is-5">Progress Bar</h5>
      <div class="inline-chart tooltip" role="tooltip" aria-label="5 / 15">
        <progress
          class="progress is-primary is-small"
          value="0.33"
          max="1">
          0.33
        </progress>
      </div>
      `,
  };
};

export let Colors = () => {
  return {
    template: hbs`
      <h5 class="title is-5">Progress Bar colors</h5>
      <div class="columns">
        <div class="column">
          <div class="inline-chart tooltip" role="tooltip" aria-label="5 / 15">
            <progress
              class="progress is-info is-small"
              value="0.33"
              max="1">
              0.33
            </progress>
          </div>
        </div>
        <div class="column">
          <div class="inline-chart tooltip" role="tooltip" aria-label="5 / 15">
            <progress
              class="progress is-success is-small"
              value="0.33"
              max="1">
              0.33
            </progress>
          </div>
        </div>
        <div class="column">
          <div class="inline-chart tooltip" role="tooltip" aria-label="5 / 15">
            <progress
              class="progress is-warning is-small"
              value="0.33"
              max="1">
              0.33
            </progress>
          </div>
        </div>
        <div class="column">
          <div class="inline-chart tooltip" role="tooltip" aria-label="5 / 15">
            <progress
              class="progress is-danger is-small"
              value="0.33"
              max="1">
              0.33
            </progress>
          </div>
        </div>
      </div>
      `,
  };
};

export let LiveUpdates = () => {
  return {
    template: hbs`
      <h5 class="title is-5">Progress Bar with live updates</h5>
      <div class="columns">
        <div class="column is-one-third">
          <div class="inline-chart tooltip" role="tooltip" aria-label="{{data.numerator}} / {{data.denominator}}">
            <progress
              class="progress is-primary is-small"
              value="{{data.percentage}}"
              max="1">
              {{data.percentage}}
            </progress>
          </div>
        </div>
      </div>
      <p class="annotation">
        <div class="boxed-section">
          <div class="boxed-section-body is-dark">
            <JsonViewer @json={{data.liveDetails}} />
          </div>
        </div>
      </p>
      `,
    context: {
      data: EmberObject.extend({
        timerTicks: 0,

        startTimer: on('init', function () {
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

        denominator: computed('timerTicks', function () {
          return Math.round(Math.random() * 1000);
        }),

        percentage: computed('timerTicks', function () {
          return Math.round(Math.random() * 100) / 100;
        }),

        numerator: computed('denominator', 'percentage', function () {
          return Math.round(this.denominator * this.percentage * 100) / 100;
        }),

        liveDetails: computed(
          'denominator',
          'numerator',
          'percentage',
          function () {
            return this.getProperties('denominator', 'numerator', 'percentage');
          }
        ),
      }).create(),
    },
  };
};
