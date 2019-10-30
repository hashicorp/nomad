/* eslint-env node */
import hbs from 'htmlbars-inline-precompile';

export default {
  title: 'Components|Breadcrumbs',
};

export const Standard = () => {
  return {
    template: hbs`
    <h5 class="title is-5">Standard breadcrumbs</h5>
      <div class="navbar is-secondary">
      <div class="navbar-item"></div>
      <nav class="breadcrumb is-large">
        <li>
          <a href="#">Topic</a>
        </li>
        <li>
          <a href="#">Sub-topic</a>
        </li>
        <li class="is-active">
          <a href="#">Active Topic</a>
        </li>
      </nav>
    </div>
    <p class='annotation'>Breadcrumbs are only ever used in the secondary nav of the primary header.</p>
    `,
  };
};

export const Single = () => {
  return {
    template: hbs`
      <h5 class="title is-5">Single breadcrumb</h5>
      <div class="navbar is-secondary">
        <div class="navbar-item"></div>
        <nav class="breadcrumb is-large">
          <li>
            <a href="#">Topic</a>
          </li>
        </nav>
      </div>
      <p class='annotation'>Breadcrumbs are given a lot of emphasis and often double as a page title. Since they are also global state, they are important for helping a user keep their bearings.</p>
  `,
  };
};
