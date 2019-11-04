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
          <li><a href="#" target="_self">Overview</a></li>
          <li><a href="#" class="is-active" target="_self">Definition</a></li>
          <li><a href="#" target="_self">Versions</a></li>
          <li><a href="#" target="_self">Deployments</a></li>
        </ul>
      </div>
          `,
  };
};

export const Single = () => {
  return {
    template: hbs`
      <h5 class="title is-5">Single page tab</h5>
      <div class="tabs">
        <ul>
          <li><a href="#" class="is-active" target="_self">Overview</a></li>
        </ul>
      </div>
          `,
  };
};
