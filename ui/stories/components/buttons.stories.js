import hbs from 'htmlbars-inline-precompile';

export default {
  title: 'Components|Buttons',
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
