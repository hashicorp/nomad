/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import hbs from 'htmlbars-inline-precompile';
import { withKnobs, optionsKnob } from '@storybook/addon-knobs';

export default {
  title: 'Components/Stepper Input',
  decorators: [withKnobs],
};

const variantKnob = () =>
  optionsKnob(
    'Variant',
    {
      Primary: 'is-primary',
      Info: 'is-info',
      Warning: 'is-warning',
      Danger: 'is-danger',
    },
    'is-primary',
    {
      display: 'inline-radio',
    },
    'variant-id'
  );

export let Standard = () => {
  return {
    template: hbs`
      <p class="mock-spacing">
        <StepperInput
          @value={{value}}
          @min={{min}}
          @max={{max}}
          @class={{variant}}
          @onChange={{action (mut value)}}>
          Stepper
        </StepperInput>
        <button class="button is-info">Button for Context</button>
      </p>
      <p class="mock-spacing"><strong>External Value:</strong> {{value}}</p>
    `,
    context: {
      min: 0,
      max: 10,
      value: 5,
      variant: variantKnob(),
    },
  };
};
