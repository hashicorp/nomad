/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

/* eslint-disable ember-a11y-testing/a11y-audit-called */
import { module, test } from 'qunit';
import { setupRenderingTest } from 'ember-qunit';
import { render, click, waitFor } from '@ember/test-helpers';
import { hbs } from 'ember-cli-htmlbars';

module('Integration | Component | trigger', function (hooks) {
  setupRenderingTest(hooks);

  module('Synchronous Interactions', function () {
    test('it can trigger a synchronous action', async function (assert) {
      this.set('name', 'Tomster');
      this.set('changeName', () => this.set('name', 'Zoey'));
      await render(hbs`
      <Trigger @do={{this.changeName}} as |trigger|>
        <h2 data-test-name>{{this.name}}</h2>
        <button data-test-button type="button" {{on "click" trigger.fns.do}}>Change my name.</button>
      </Trigger>
      `);
      assert
        .dom('[data-test-name]')
        .hasText('Tomster', 'Initial state renders correctly.');

      await click('[data-test-button]');

      assert
        .dom('[data-test-name]')
        .hasText(
          'Zoey',
          'The name property changes when the button is clicked'
        );
    });

    test('it sets the result of the action', async function (assert) {
      this.set('tomster', () => 'Tomster');
      await render(hbs`
      <Trigger @do={{this.tomster}} as |trigger|>
        {{#if trigger.data.result}}
          <h2 data-test-name>{{trigger.data.result}}</h2>
        {{/if}}
        <button data-test-button {{on "click" trigger.fns.do}}>Generate</button>
      </Trigger>
      `);
      assert
        .dom('[data-test-name]')
        .doesNotExist(
          'Initial state does not render because there is no result yet.'
        );

      await click('[data-test-button]');

      assert
        .dom('[data-test-name]')
        .hasText(
          'Tomster',
          'The result state updates after the triggered action'
        );
    });
  });

  module('Asynchronous Interactions', function () {
    test('it can trigger an asynchronous action', async function (assert) {
      this.set(
        'onTrigger',
        () =>
          new Promise((resolve) => {
            this.set('resolve', resolve);
          })
      );

      await render(hbs`
      <Trigger @do={{this.onTrigger}} as |trigger|>
        {{#if trigger.data.isBusy}}
          <div data-test-div-loading>...Loading</div>
        {{/if}}
        {{#if trigger.data.isSuccess}}
          <div data-test-div>Success!</div>
        {{/if}}
        <button data-test-button {{on "click" trigger.fns.do}}>Click Me</button>
      </Trigger>
      `);

      assert
        .dom('[data-test-div]')
        .doesNotExist(
          'The div does not render until after the action dispatches successfully'
        );

      await click('[data-test-button]');
      assert
        .dom('[data-test-div-loading]')
        .exists(
          'Loading state is displayed when the action hasnt resolved yet'
        );
      assert
        .dom('[data-test-div]')
        .doesNotExist(
          'Success message does not display until after promise resolves'
        );

      this.resolve();
      await waitFor('[data-test-div]');
      assert
        .dom('[data-test-div-loading]')
        .doesNotExist(
          'Loading state is no longer rendered after state changes from busy to success'
        );
      assert
        .dom('[data-test-div]')
        .exists(
          'Action has dispatched successfully after the promise resolves'
        );

      await click('[data-test-button]');
      assert
        .dom('[data-test-div]')
        .doesNotExist(
          'Aftering clicking the button, again, the state is reset'
        );
      assert
        .dom('[data-test-div-loading]')
        .exists(
          'After clicking the button, again, we are back in the loading state'
        );

      this.resolve();
      await waitFor('[data-test-div]');

      assert
        .dom('[data-test-div]')
        .exists(
          'An new action and new promise resolve after clicking the button for the second time'
        );
    });

    test('it handles the success state', async function (assert) {
      this.set(
        'onTrigger',
        () =>
          new Promise((resolve) => {
            this.set('resolve', resolve);
          })
      );
      this.set('onSuccess', () => assert.step('On success happened'));

      await render(hbs`
      <Trigger @do={{this.onTrigger}} @onSuccess={{this.onSuccess}} as |trigger|>
          {{#if trigger.data.isSuccess}}
            <span data-test-div>Success!</span>
          {{/if}}
        <button data-test-button {{on "click" trigger.fns.do}}>Click Me</button>
      </Trigger>
      `);

      assert
        .dom('[data-test-div]')
        .doesNotExist(
          'No text should appear until after the onSuccess callback is fired'
        );
      await click('[data-test-button]');
      this.resolve();
      await waitFor('[data-test-div]');
      assert.verifySteps(['On success happened']);
    });

    test('it handles the error state', async function (assert) {
      this.set(
        'onTrigger',
        () =>
          new Promise((_, reject) => {
            this.set('reject', reject);
          })
      );
      this.set('onError', () => {
        assert.step('On error happened');
      });

      await render(hbs`
      <Trigger @do={{this.onTrigger}} @onError={{this.onError}} as |trigger|>
          {{#if trigger.data.isBusy}}
            <div data-test-div-loading>...Loading</div>
          {{/if}}
          {{#if trigger.data.isError}}
            <span data-test-span>Error!</span>
          {{/if}}
        <button data-test-button {{on "click" trigger.fns.do}}>Click Me</button>
      </Trigger>
      `);

      await click('[data-test-button]');
      assert
        .dom('[data-test-div-loading]')
        .exists(
          'Loading state is displayed when the action hasnt resolved yet'
        );

      assert
        .dom('[data-test-div]')
        .doesNotExist(
          'No text should appear until after the onError callback is fired'
        );

      this.reject();
      await waitFor('[data-test-span]');
      assert.verifySteps(['On error happened']);

      await click('[data-test-button]');

      assert
        .dom('[data-test-div-loading]')
        .exists(
          'The previous error state was cleared and we show loading, again.'
        );

      assert.dom('[data-test-div]').doesNotExist('The error state is cleared');

      this.reject();
      await waitFor('[data-test-span]');
      assert.verifySteps(['On error happened'], 'The error dispatches');
    });
  });
});
