/* eslint-env node */
import hbs from 'htmlbars-inline-precompile';
import notes from './two-step-button.md';

export default {
  title: 'Components|Two-Step Button',
  parameters: {
    notes,
  },
};

export const TwoStepButton = () => {
  return {
    template: hbs`
      <h5 class="title is-5">Two Step Button</h5>
      <br><br>
      <TwoStepButton
        @idleText="Scary Action"
        @cancelText="Nvm"
        @confirmText="Yep"
        @confirmationMessage="Wait, really? Like...seriously?"
      />
      `,
  };
};

export const InTitle = () => {
  return {
    template: hbs`
      <h5 class="title is-5">Two Step Button in title</h5>
      <br><br>
      <h1 class="title">
        This is a page title
        <TwoStepButton
          @idleText="Scary Action"
          @cancelText="Nvm"
          @confirmText="Yep"
          @confirmationMessage="Wait, really? Like...seriously?"
        />
      </h1>
          `,
  };
};

export const LoadingState = () => {
  return {
    template: hbs`
    <h5 class="title is-5">Two Step Button loading state</h5>
    <br><br>
    <h1 class="title">
      This is a page title
      <TwoStepButton
        @idleText="Scary Action"
        @cancelText="Nvm"
        @confirmText="Yep"
        @confirmationMessage="Wait, really? Like...seriously?"
        @awaitingConfirmation={{true}}
        @state="prompt"
      />
    </h1>
    <p class='annotation'>  <strong>Note:</strong> the <code>state</code> property is internal state and only used here to bypass the idle state for demonstration purposes.</p>
    `,
  };
};
