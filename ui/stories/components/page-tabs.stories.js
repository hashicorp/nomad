/* eslint-env node */
import hbs from 'htmlbars-inline-precompile';

export default {
  title: 'Components|Page Tabs',
};

export const Standard = () => {
  return {
    template: hbs`
      <h5 class="title is-5">Page tabs</h5>
      <div class="tabs">
        <ul>
          <li><a href="#">Overview</a></li>
          <li><a href="#" class="is-active">Definition</a></li>
          <li><a href="#">Versions</a></li>
          <li><a href="#">Deployments</a></li>
        </ul>
      </div>
      <!-- FIXME clicking navigates to the iframe! -->
          `,
  };
};

export const Single = () => {
  return {
    template: hbs`
      <h5 class="title is-5">Single page tab</h5>
      <div class="tabs">
        <ul>
          <li><a href="#" class="is-active">Overview</a></li>
        </ul>
      </div>
          `,
  };
};
