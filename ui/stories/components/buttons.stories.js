/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import hbs from 'htmlbars-inline-precompile';

export default {
  title: 'Components/Buttons',
};

export let Standard = () => {
  return {
    template: hbs`
      <h5 class="title is-5">Buttons</h5>
      <div class="block">
        <a class="button">Button</a>
        <a class="button is-white">White</a>
        <a class="button is-light">Light</a>
        <a class="button is-dark">Dark</a>
        <a class="button is-black">Black</a>
        <a class="button is-link">Link</a>
      </div>
      <div class="block">
        <a class="button is-primary">Primary</a>
        <a class="button is-info">Info</a>
        <a class="button is-success">Success</a>
        <a class="button is-warning">Warning</a>
        <a class="button is-danger">Danger</a>
      </div>
      `,
  };
};

export let Outline = () => {
  return {
    template: hbs`
      <h5 class="title is-5">Outline buttons</h5>
      <div class="block">
        <a class="button is-outlined">Outlined</a>
        <a class="button is-primary is-outlined">Primary</a>
        <a class="button is-info is-outlined">Info</a>
        <a class="button is-success is-outlined">Success</a>
        <a class="button is-warning is-outlined">Warning</a>
        <a class="button is-danger is-outlined is-important">Danger</a>
      </div>
      `,
  };
};

export let Hollow = () => {
  return {
    template: hbs`
      <h5 class="title is-5">Hollow buttons</h5>
      <div class="block" style="background:#25BA81; padding:30px">
        <a class="button is-primary is-inverted is-outlined">Primary</a>
        <a class="button is-info is-inverted is-outlined">Info</a>
        <a class="button is-success is-inverted is-outlined">Success</a>
        <a class="button is-warning is-inverted is-outlined">Warning</a>
        <a class="button is-danger is-inverted is-outlined">Danger</a>
      </div>
      `,
  };
};

export let Sizes = () => {
  return {
    template: hbs`
      <h5 class="title is-5">Button sizes</h5>
      <div class="block">
        <a class="button is-small">Small</a>
        <a class="button">Normal</a>
        <a class="button is-medium">Medium</a>
        <a class="button is-large">Large</a>
      </div>
      `,
  };
};

export let Disabled = () => {
  return {
    template: hbs`
      <h5 class="title is-5">Anchor elements as buttons</h5>
      <div class="block">
        <a class="button is-disabled">Button</a>
        <a class="button is-white is-disabled">White</a>
        <a class="button is-light is-disabled">Light</a>
        <a class="button is-dark is-disabled">Dark</a>
        <a class="button is-black is-disabled">Black</a>
        <a class="button is-link is-disabled">Link</a>
      </div>
      <div class="block">
        <a class="button is-primary is-disabled">Primary</a>
        <a class="button is-info is-disabled">Info</a>
        <a class="button is-success is-disabled">Success</a>
        <a class="button is-warning is-disabled">Warning</a>
        <a class="button is-danger is-disabled">Danger</a>
      </div>

      <h5 class="title is-5">Button elements with <code>disabled</code> attribute</h5>
      <div class="block">
        <button class="button is-disabled" disabled>Button</button>
        <button class="button is-white is-disabled" disabled>White</button>
        <button class="button is-light is-disabled" disabled>Light</button>
        <button class="button is-dark is-disabled" disabled>Dark</button>
        <button class="button is-black is-disabled" disabled>Black</button>
        <button class="button is-link is-disabled" disabled>Link</button>
      </div>
      <div class="block">
        <button class="button is-primary is-disabled" disabled>Primary</button>
        <button class="button is-info is-disabled" disabled>Info</button>
        <button class="button is-success is-disabled" disabled>Success</button>
        <button class="button is-warning is-disabled" disabled>Warning</button>
        <button class="button is-danger is-disabled" disabled>Danger</button>
      </div>

      <h5 class="title is-5">Button elements with <code>aria-disabled="true"</code></h5>
      <div class="block">
        <button class="button is-disabled" aria-disabled="true">Button</button>
        <button class="button is-white is-disabled" aria-disabled="true">White</button>
        <button class="button is-light is-disabled" aria-disabled="true">Light</button>
        <button class="button is-dark is-disabled" aria-disabled="true">Dark</button>
        <button class="button is-black is-disabled" aria-disabled="true">Black</button>
        <button class="button is-link is-disabled" aria-disabled="true">Link</button>
      </div>
      <div class="block">
        <button class="button is-primary is-disabled" aria-disabled="true">Primary</button>
        <button class="button is-info is-disabled" aria-disabled="true">Info</button>
        <button class="button is-success is-disabled" aria-disabled="true">Success</button>
        <button class="button is-warning is-disabled" aria-disabled="true">Warning</button>
        <button class="button is-danger is-disabled" aria-disabled="true">Danger</button>
      </div>
      `,
  };
};
