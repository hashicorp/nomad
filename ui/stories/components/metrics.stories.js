/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import hbs from 'htmlbars-inline-precompile';

export default {
  title: 'Components/Metrics',
};

export let Standard = () => {
  return {
    template: hbs`
      <h5 class="title is-5">Metrics</h5>
      <div class="metric-group">
        <div class="metric">
          <h3 class="label">Label</h3>
          <p class="value">12</p>
        </div>
      </div>
      <p class="annotation">Metrics are a way to show simple values (generally numbers). Labels are smaller than numbers to put emphasis on the data.</p>
      `,
  };
};

export let Groups = () => {
  return {
    template: hbs`
      <h5 class="title is-5">Metric groups</h5>
      <div class="metric-group">
        <div class="metric">
          <h3 class="label">Label</h3>
          <p class="value">1 / 2</p>
        </div>
        <div class="metric">
          <h3 class="label">Number</h3>
          <p class="value">1,300</p>
        </div>
        <div class="metric">
          <h3 class="label">Datacenter</h3>
          <p class="value">dc1</p>
        </div>
      </div>

      <div class="metric-group">
        <div class="metric">
          <h3 class="label">Today</h3>
          <p class="value">81º</p>
        </div>
        <div class="metric">
          <h3 class="label">Tomorrow</h3>
          <p class="value">73º</p>
        </div>
      </div>
      <p class="annotation">Related metrics should be lumped together in metric groups. All metrics have to be in a metric group. By putting multiple metrics in a single group, they will be visually lumped together.</p>
      `,
  };
};

export let Colors = () => {
  return {
    template: hbs`
      <h5 class="title is-5">Metric colors</h5>
      <div class="metric-group">
        <div class="metric is-info">
          <h3 class="label">Info</h3>
          <p class="value">1</p>
        </div>
        <div class="metric is-success">
          <h3 class="label">Success</h3>
          <p class="value">2</p>
        </div>
        <div class="metric is-warning">
          <h3 class="label">Warning</h3>
          <p class="value">3</p>
        </div>
        <div class="metric is-danger">
          <h3 class="label">Danger</h3>
          <p class="value">4</p>
        </div>
      </div>

      <div class="metric-group">
        <div class="metric is-white">
          <h3 class="label">White</h3>
          <p class="value">5</p>
        </div>
        <div class="metric is-light">
          <h3 class="label">Light</h3>
          <p class="value">6</p>
        </div>
        <div class="metric is-primary">
          <h3 class="label">Primary</h3>
          <p class="value">7</p>
        </div>
        <div class="metric is-dark">
          <h3 class="label">Dark</h3>
          <p class="value">8</p>
        </div>
        <div class="metric is-black">
          <h3 class="label">Black</h3>
          <p class="value">9</p>
        </div>
      </div>
      <p class="annotation">All color-modifiers work for metrics, but some work better than others.</p>
      <p class="annotation">Emotive colors work well and are put to use when applicable. Other colors have worse support and less utility.</p>
      `,
  };
};

export let States = () => {
  return {
    template: hbs`
      <h5 class="title is-5">Metric states</h5>
      <div class="metric-group">
        <div class="metric is-primary is-faded">
          <h3 class="label">One</h3>
          <p class="value">A</p>
        </div>
        <div class="metric is-primary">
          <h3 class="label">Two</h3>
          <p class="value">B</p>
        </div>
        <div class="metric is-primary is-faded">
          <h3 class="label">Three</h3>
          <p class="value">C</p>
        </div>
      </div>

      <div class="metric-group">
        <div class="metric is-danger is-faded">
          <h3 class="label">One</h3>
          <p class="value">A</p>
        </div>
        <div class="metric is-danger is-faded">
          <h3 class="label">Two</h3>
          <p class="value">B</p>
        </div>
        <div class="metric is-danger">
          <h3 class="label">Three</h3>
          <p class="value">C</p>
        </div>
      </div>
      <p class="annotation">Metrics have a disabled state. This is used when a metric is non-existent or irrelevant. It's just as important to show the lack of value as it is to show a value, so simply not rendering non-existent or irrelevant metrics would be worse.</p>
      `,
  };
};
