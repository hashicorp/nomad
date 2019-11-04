/* eslint-env node */
import hbs from 'htmlbars-inline-precompile';

export default {
  title: 'Components|Page Title',
};

export const Standard = () => {
  return {
    template: hbs`
      <h5 class="title is-5">Page Title</h5>
      <div class="mock-spacing">
        <h1 class="title">This is the Page Title</h1>
      </div>
      <p class='annotation'>In its simplest form, a page title is just an H1.</p>
    `,
  };
};

export const AfterElements = () => {
  return {
    template: hbs`
      <h5 class="title is-5">Page Title With After Elements</h5>
      <div class="mock-spacing">
        <h1 class="title">
          This is the Page Title
          <span class="bumper-left tag is-running">Running</span>
          <span class="tag is-hollow is-small no-text-transform">237aedcb8982fe09bcee0877acedd</span>
        </h1>
      </div>
      <p class='annotation'>It is common to put high-impact tags and badges to the right of titles. These tags should only ever appear on the right-hand side of the title, and they should be listed in descending weights. Tags with a background are heavier than tags that are hollow. Longer values are heavier than shorter values.</p>
        `,
  };
};

export const StatusLight = () => {
  return {
    template: hbs`
      <h5 class="title is-5">Page Title With Status Light</h5>
      <div class="mock-spacing">
        <h1 class="title">
          <span class="node-status-light initializing"></span>
          This is the Page Title
          <span class="bumper-left tag is-running">Running</span>
          <span class="tag is-hollow is-small no-text-transform">237aedcb8982fe09bcee0877acedd</span>
        </h1>
      </div>
      <p class='annotation'>A simple color or pattern is faster to scan than a title and can often say more than words can. For pages that have an important status component to them (e.g., client detail page), a status light can be shown to the left of the title where typically eyes will begin to scan a page.</p>
        `,
  };
};

export const Actions = () => {
  return {
    template: hbs`
      <h5 class="title is-5">PageTitleWithActions</h5>
      <div class="mock-spacing">
        <h1 class="title">
          <span class="node-status-light initializing"></span>
          This is the Page Title
          <span class="bumper-left tag is-running">Running</span>
          <span class="tag is-hollow is-small no-text-transform">237aedcb8982fe09bcee0877acedd</span>
          <button class="button is-warning is-small is-inline">If you wish</button>
          <button class="button is-danger is-outlined is-important is-small is-inline">No Regrets</button>
        </h1>
      </div>
      <p class='annotation'>When actions apply to the entire context of a page, (e.g., job actions on the job detail page), buttons for these actions go in the page title. Buttons are always placed on the far right end of a page title. No elements can go to the right of these buttons.</p>
        `,
  };
};
