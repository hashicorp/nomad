/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import hbs from 'htmlbars-inline-precompile';

export default {
  title: 'Components/Alerts',
};

export let Standard = () => {
  return {
    template: hbs`
      <h5 class="title is-5">Alert</h5>
      <div class="notification is-info">
        <h3 class="title is-4">This is an alert</h3>
        <p>Alerts are used for both situational and reactionary information.</p>
      </div>
      <p class="annotation">Alerts use Bulma's notification component.</p>
      `,
  };
};

export let Colors = () => {
  return {
    template: hbs`
      <h5 class="title is-5">Alert colors</h5>
      <div class="notification is-info">
        <h3 class="title is-4">This is an alert</h3>
        <p>Alerts are used for both situational and reactionary information.</p>
      </div>

      <div class="notification is-success">
        <h3 class="title is-4">This is an alert</h3>
        <p>Alerts are used for both situational and reactionary information.</p>
      </div>

      <div class="notification is-warning">
        <h3 class="title is-4">This is an alert</h3>
        <p>Alerts are used for both situational and reactionary information.</p>
      </div>

      <div class="notification is-danger">
        <h3 class="title is-4">This is an alert</h3>
        <p>Alerts are used for both situational and reactionary information.</p>
      </div>

      <p class="annotation">Alerts are always paired with an emotive color. If there is no emotive association with the content of the alert, then an alert is the wrong component to use.</p>
      `,
  };
};

export let Dismissal = () => {
  return {
    template: hbs`
    <h5 class="title is-5">Alert dismissal</h5>
    <div class="notification is-info">
      <div class="columns">
        <div class="column">
          <h3 class="title is-4">This is an alert</h3>
          <p>Alerts are used for both situational and reactionary information.</p>
        </div>
        <div class="column is-centered is-minimum">
          <button class="button is-info">Okay</button>
        </div>
      </div>
    </div>

    <div class="notification is-success">
      <div class="columns">
        <div class="column">
          <h3 class="title is-4">This is an alert</h3>
          <p>Alerts are used for both situational and reactionary information.</p>
        </div>
        <div class="column is-centered is-minimum">
          <button class="button is-success">Okay</button>
        </div>
      </div>
    </div>

    <div class="notification is-warning">
      <div class="columns">
        <div class="column">
          <h3 class="title is-4">This is an alert</h3>
          <p>Alerts are used for both situational and reactionary information.</p>
        </div>
        <div class="column is-centered is-minimum">
          <button class="button is-warning">Okay</button>
        </div>
      </div>
    </div>

    <div class="notification is-danger">
      <div class="columns">
        <div class="column">
          <h3 class="title is-4">This is an alert</h3>
          <p>Alerts are used for both situational and reactionary information.</p>
        </div>
        <div class="column is-centered is-minimum">
          <button class="button is-danger">Okay</button>
        </div>
      </div>
    </div>
    `,
  };
};
