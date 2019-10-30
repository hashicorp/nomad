/* eslint-env node */
import hbs from 'htmlbars-inline-precompile';

export default {
  title: 'Components|Inline Definitions',
};

export const Basic = () => {
  return {
    template: hbs`
      <h5 class="title is-5">Basic Inline Definitions</h5>
      <div class="boxed-section is-small">
        <div class="boxed-section-body inline-definitions">
          <span class="label">Some Label</span>
          <span class="pair">
            <span class="term">Term Name</span>
            <span>Term Value</span>
          </span>
          <span class="pair">
            <span class="term">Running?</span>
            <span>Yes</span>
          </span>
          <span class="pair">
            <span class="term">Last Updated</span>
            <span>{{format-ts (now)}}</span>
          </span>
        </div>
      </div>
      <p class='annotation'>A way to tightly display key/value information. Typically seen at the top of pages.</p>
        `,
  };
};

export const Variants = () => {
  return {
    template: hbs`
      <h5 class="title is-5">Inline Definitions Variants</h5>
      <div class="boxed-section is-small is-success">
        <div class="boxed-section-body inline-definitions">
          <span class="label">Success Label</span>
          <span class="pair">
            <span class="term">Term Name</span>
            <span>Term Value</span>
          </span>
          <span class="pair">
            <span class="term">Last Updated</span>
            <span>{{format-ts (now)}}</span>
          </span>
        </div>
      </div>
      <div class="boxed-section is-small is-warning">
        <div class="boxed-section-body inline-definitions">
          <span class="label">Warning Label</span>
          <span class="pair">
            <span class="term">Term Name</span>
            <span>Term Value</span>
          </span>
          <span class="pair">
            <span class="term">Last Updated</span>
            <span>{{format-ts (now)}}</span>
          </span>
        </div>
      </div>
      <div class="boxed-section is-small is-danger">
        <div class="boxed-section-body inline-definitions">
          <span class="label">Danger Label</span>
          <span class="pair">
            <span class="term">Term Name</span>
            <span>Term Value</span>
          </span>
          <span class="pair">
            <span class="term">Last Updated</span>
            <span>{{format-ts (now)}}</span>
          </span>
        </div>
      </div>
      <div class="boxed-section is-small is-info">
        <div class="boxed-section-body inline-definitions">
          <span class="label">Info Label</span>
          <span class="pair">
            <span class="term">Term Name</span>
            <span>Term Value</span>
          </span>
          <span class="pair">
            <span class="term">Last Updated</span>
            <span>{{format-ts (now)}}</span>
          </span>
        </div>
      </div>
        <p class='annotation'>Inline definitions are meant to pair well with emotive color variations.</p>
        `,
  };
};
