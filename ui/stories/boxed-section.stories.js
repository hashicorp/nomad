/* eslint-env node */
import hbs from 'htmlbars-inline-precompile';
import { storiesOf } from '@storybook/ember';
import { withKnobs, optionsKnob } from '@storybook/addon-knobs';

storiesOf('Boxed section/', module)
  .addParameters({ options: { showPanel: true } })
  .addDecorator(withKnobs())
  .add(`Boxed section`, () => ({
    template: hbs`
    <h5 class="title is-5">Boxed section</h5>
    <div class="boxed-section {{variant}}">
      <div class="boxed-section-head">
        Boxed Section
      </div>
      <div class="boxed-section-body">
        <div class="mock-content">
          <div class="mock-image"></div>
          <div class="mock-copy"></div>
          <div class="mock-copy"></div>
        </div>
      </div>
    </div>
    `,
    context: contextFactory('something'),
  }),
  {})
  .add(`Boxed section with right hand details`, () => ({
    template: hbs`
      <h5 class="title is-5">Boxed section with right hand details</h5>
      <div class="boxed-section {{variant}}">
        <div class="boxed-section-head">
          Boxed Section With Right Hand Details
          <span class="pull-right">{{now interval=1000}}</span>
        </div>
        <div class="boxed-section-body">
          <div class="mock-content">
            <div class="mock-image"></div>
            <div class="mock-copy"></div>
            <div class="mock-copy"></div>
          </div>
        </div>
      </div>
  `,
    context: contextFactory('something'),
  }),
  {}
)
.add(`Boxed section with title decoration`, () => ({
  template: hbs`
    <h5 class="title is-5">Boxed section with title decoration</h5>
    <div class="boxed-section {{variant}}">
      <div class="boxed-section-head">
        Boxed Section With Title Decoration
        <span class="badge is-white">7</span>
      </div>
      <div class="boxed-section-body">
        <div class="mock-content">
          <div class="mock-image"></div>
          <div class="mock-copy"></div>
          <div class="mock-copy"></div>
        </div>
      </div>
    </div>
`,
  context: contextFactory(),
}),
{}
)
.add(`Boxed section with foot`, () => ({
  template: hbs`
    <h5 class="title is-5">Boxed section with foot</h5>
    <div class="boxed-section {{variant}}">
      <div class="boxed-section-head">
        Boxed Section With Large Header
      </div>
      <div class="boxed-section-body with-foot">
        <div class="mock-content">
          <div class="mock-image"></div>
          <div class="mock-copy"></div>
          <div class="mock-copy"></div>
        </div>
      </div>
      <div class="boxed-section-foot">
        <span>Left-aligned message</span>
        <a href="#" class="pull-right">Toggle or other action</a>
      </div>
    </div>
`,
  context: contextFactory(),
}),
{}
)
.add(`Boxed section with large header`, () => ({
  template: hbs`
    <h5 class="title is-5">Boxed section with large header</h5>
    <div class="boxed-section {{variant}}">
      <div class="boxed-section-head">
        <div class="boxed-section-row">
          Boxed Section With Large Header
          <span class="badge is-white is-subtle bumper-left">Status</span>
        </div>
        <div class="boxed-section-row">
          <span class="tag is-outlined">A tag that goes on a second line because it's rather long</span>
        </div>
      </div>
      <div class="boxed-section-body">
        <div class="mock-content">
          <div class="mock-image"></div>
          <div class="mock-copy"></div>
          <div class="mock-copy"></div>
        </div>
      </div>
    </div>
`,
  context: contextFactory(),
}),
{}
)
.add(`Boxed section with dark body`, () => ({
  template: hbs`
    <h5 class="title is-5">Boxed section with dark body</h5>
    <div class="boxed-section {{variant.slug}}">
      <div class="boxed-section-head">
        Boxed Section With Dark Body
      </div>
      <div class="boxed-section-body is-dark">
        <div class="mock-content">
          <div class="mock-image"></div>
          <div class="mock-copy"></div>
          <div class="mock-copy"></div>
        </div>
      </div>
    </div>
`,
  context: contextFactory(),
}),
{}
);

// FIXME when this was just a const, it caused no knobs to display…?
function contextFactory() {
  return   {variant: optionsKnob('Variant', {
    Normal: '',
    Info: 'is-info',
    Warning: 'is-warning',
    Danger: 'is-danger',
  }, '', {
    display: 'inline-radio',
  }, 'variant-id')
  };
}
