import hbs from 'htmlbars-inline-precompile';

export default {
  title: 'Components|Stepper Input',
};

export let Standard = () => {
  return {
    template: hbs`
      <p class="mock-spacing">
        <StepperInput
          @value={{value}}
          @min={{min}}
          @max={{max}}
          @onChange={{action (mut value)}}>
          Stepper
        </StepperInput>
      </p>
      <p class="mock-spacing"><strong>External Value:</strong> {{value}}</p>
    `,
    context: {
      min: 0,
      max: 10,
      value: 5,
    },
  };
};
