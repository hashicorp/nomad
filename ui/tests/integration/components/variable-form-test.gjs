/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import { module, test } from 'qunit';
import { setupRenderingTest } from 'ember-qunit';
import { tracked } from '@glimmer/tracking';
import { componentA11yAudit } from 'nomad-ui/tests/helpers/a11y-audit';
import {
  click,
  typeIn,
  triggerEvent,
  findAll,
  render,
  settled,
} from '@ember/test-helpers';
import { setupMirage } from 'ember-cli-mirage/test-support';
import setupCodeMirror from 'nomad-ui/tests/helpers/codemirror';
import { codeFillable, code } from 'nomad-ui/tests/pages/helpers/codemirror';
import percySnapshot from '@percy/ember';
import { clickToggle, clickOption } from 'nomad-ui/tests/helpers/helios';

import faker from 'nomad-ui/mirage/faker';
import VariableForm from 'nomad-ui/components/variable-form';

function pickInteractiveControl(scope, selectors) {
  const root = document.querySelector(scope);
  if (!root) return null;

  const controls = selectors.flatMap((selector) =>
    Array.from(root.querySelectorAll(selector)),
  );

  const interactive = controls.filter((control) => {
    if (!control) return false;
    if (control.disabled) return false;
    if (control.type === 'hidden') return false;
    return control.offsetParent !== null || control.getClientRects().length > 0;
  });

  return interactive[0] || controls[0] || null;
}

function findKeyControl(scope = '.key-value:last-of-type') {
  return pickInteractiveControl(scope, [
    'input[data-test-var-key]',
    'textarea[data-test-var-key]',
    '[data-test-var-key] input',
    '[data-test-var-key] textarea',
  ]);
}

function findValueControl(scope = '.key-value:last-of-type') {
  return pickInteractiveControl(scope, [
    'textarea[data-test-var-value]',
    'input[data-test-var-value]',
    '[data-test-var-value] textarea',
    '[data-test-var-value] input',
  ]);
}

class State {
  @tracked view = 'table';
}

module('Integration | Component | variable-form', function (hooks) {
  setupRenderingTest(hooks);
  setupMirage(hooks);
  setupCodeMirror(hooks);

  test('passes an accessibility audit', async function (assert) {
    const mockedModel = this.server.create('variable', {
      keyValues: [{ key: '', value: '' }],
    });
    await render(<template><VariableForm @model={{mockedModel}} /></template>);
    await componentA11yAudit(this.element, assert);
  });

  test('shows a single row by default and modifies on "Add More" and "Delete"', async function (assert) {
    const mockedModel = this.server.create('variable', {
      keyValues: [{ key: '', value: '' }],
    });

    await render(<template><VariableForm @model={{mockedModel}} /></template>);
    assert.deepEqual(
      findAll('div.key-value').length,
      1,
      'A single KV row exists by default',
    );

    assert
      .dom('[data-test-add-kv]')
      .isDisabled(
        'The "Add More" button is disabled until key and value are filled',
      );

    await typeIn('[data-test-var-key]', 'foo');

    assert
      .dom('[data-test-add-kv]')
      .isDisabled(
        'The "Add More" button is still disabled with only key filled',
      );

    await typeIn('[data-test-var-value]', 'bar');

    assert
      .dom('[data-test-add-kv]')
      .isNotDisabled(
        'The "Add More" button is no longer disabled after key and value are filled',
      );

    await click('[data-test-add-kv]');

    assert.deepEqual(
      findAll('div.key-value').length,
      2,
      'A second KV row exists after adding a new one',
    );

    await typeIn('.key-value:last-of-type [data-test-var-key]', 'foo');
    await typeIn('.key-value:last-of-type [data-test-var-value]', 'bar');
    await click('[data-test-add-kv]');

    assert.deepEqual(
      findAll('div.key-value').length,
      3,
      'A third KV row exists after adding a new one',
    );

    await click('.delete-entry-button');

    assert.deepEqual(
      findAll('div.key-value').length,
      2,
      'Back down to two rows after hitting delete',
    );
  });

  module('editing and creating new key/value pairs', function () {
    test('it should allow each key/value row to toggle password visibility', async function (assert) {
      faker.seed(1);
      const mockedModel = this.server.create('variable', {
        keyValues: [{ key: 'foo', value: 'bar' }],
      });

      await render(
        <template><VariableForm @model={{mockedModel}} /></template>,
      );
      await click('[data-test-add-kv]');

      findAll('.value-label').forEach((label, index) => {
        const maskedInput = label.querySelector('.hds-form-masked-input');
        assert.ok(
          maskedInput.classList.contains('hds-form-masked-input--is-masked'),
          `Value ${index + 1} is hidden by default`,
        );
      });

      await click('.hds-form-visibility-toggle');
      const [firstRow, secondRow] = findAll('.hds-form-masked-input');

      assert.ok(
        firstRow.classList.contains('hds-form-masked-input--is-not-masked'),
        'Only the row that is clicked on toggles visibility',
      );
      assert.ok(
        secondRow.classList.contains('hds-form-masked-input--is-masked'),
        'Rows that are not clicked remain obscured',
      );

      await click('.hds-form-visibility-toggle');
      assert.ok(
        firstRow.classList.contains('hds-form-masked-input--is-masked'),
        'Only the row that is clicked on toggles visibility',
      );
      assert.ok(
        secondRow.classList.contains('hds-form-masked-input--is-masked'),
        'Rows that are not clicked remain obscured',
      );
      await percySnapshot(assert);
    });
  });

  test('Existing variable shows properties by default', async function (assert) {
    const keyValues = [
      { key: 'my-completely-normal-key', value: 'never' },
      { key: 'another key, but with spaces', value: 'gonna' },
      { key: 'once/more/with/slashes', value: 'give' },
      { key: 'and_some_underscores', value: 'you' },
      { key: 'and\\now/for-something_completely@different', value: 'up' },
    ];

    const mockedModel = this.server.create('variable', {
      path: 'my/path/to',
      keyValues,
    });
    await render(<template><VariableForm @model={{mockedModel}} /></template>);
    assert.deepEqual(
      findAll('div.key-value').length,
      5,
      'Shows 5 existing key values',
    );
    assert.deepEqual(
      findAll('.delete-entry-button').length,
      5,
      'Shows "delete" for all five rows',
    );
    assert.deepEqual(
      findAll('[data-test-add-kv]').length,
      1,
      'Shows "add more" only on the last row',
    );

    findAll('div.key-value').forEach((row, index) => {
      assert.deepEqual(
        row.querySelector(`[data-test-var-key]`).value,
        keyValues[index].key,
        `Key ${index + 1} is correct`,
      );

      assert.deepEqual(
        row.querySelector(`[data-test-var-value]`).value,
        keyValues[index].value,
        keyValues[index].value,
      );
    });
  });

  test('Prevent editing path input on existing variables', async function (assert) {
    const variable = await this.server.create('variable', {
      name: 'foo',
      namespace: 'bar',
      path: '/baz/bat',
      keyValues: [{ key: '', value: '' }],
    });
    variable.isNew = false;
    await render(<template><VariableForm @model={{variable}} /></template>);
    assert.dom('[data-test-path-input]').hasValue('/baz/bat', 'Path is set');
    assert
      .dom('[data-test-path-input]')
      .isDisabled('Existing variable is in disabled state');

    variable.isNew = true;
    variable.path = '';
    await render(<template><VariableForm @model={{variable}} /></template>);
    assert
      .dom('[data-test-path-input]')
      .isNotDisabled('New variable is not in disabled state');
  });

  module('Validation', function () {
    test('warns when you try to create a path that already exists', async function (assert) {
      this.server.createList('namespace', 3);

      const mockedModel = this.server.create('variable', {
        path: '',
        keyValues: [{ key: '', value: '' }],
      });

      this.server.create('variable', {
        path: 'baz/bat',
      });
      this.server.create('variable', {
        path: 'baz/bat/qux',
        namespace: this.server.db.namespaces[2].id,
      });

      const existingVariables = this.server.db.variables.toArray();

      await render(
        <template>
          <VariableForm
            @model={{mockedModel}}
            @existingVariables={{existingVariables}}
          />
        </template>,
      );

      await typeIn('[data-test-path-input]', 'foo/bar');
      assert.dom('[data-test-duplicate-variable-error]').doesNotExist();
      assert
        .dom('[data-test-path-input]')
        .doesNotHaveClass('hds-form-text-input--is-invalid');

      document.querySelector('[data-test-path-input]').value = '';
      await typeIn('[data-test-path-input]', 'baz/bat');

      assert.dom('[data-test-duplicate-variable-error]').exists();
      assert
        .dom('[data-test-path-input]')
        .hasClass('hds-form-text-input--is-invalid');

      await clickToggle('[data-test-variable-namespace-filter]');
      await clickOption(
        '[data-test-variable-namespace-filter]',
        this.server.db.namespaces[2].id,
      );
      assert.dom('[data-test-duplicate-variable-error]').doesNotExist();
      assert
        .dom('[data-test-path-input]')
        .doesNotHaveClass('hds-form-text-input--is-invalid');

      document.querySelector('[data-test-path-input]').value = '';
      await typeIn('[data-test-path-input]', 'baz/bat/qux');
      assert.dom('[data-test-duplicate-variable-error]').exists();
      assert
        .dom('[data-test-path-input]')
        .hasClass('hds-form-text-input--is-invalid');
    });

    test('warns when you try to create a path with invalid characters', async function (assert) {
      this.server.createList('namespace', 3);

      const mockedModel = this.server.create('variable', {
        path: '',
        keyValues: [{ key: '', value: '' }],
      });

      await render(
        <template><VariableForm @model={{mockedModel}} /></template>,
      );

      await typeIn('[data-test-path-input]', 'foo-bar');
      assert.dom('[data-test-invalid-path-error]').doesNotExist();
      assert
        .dom('[data-test-path-input]')
        .doesNotHaveClass('hds-form-text-input--is-invalid');

      document.querySelector('[data-test-path-input]').value = '';
      await typeIn('[data-test-path-input]', 'foo bar');

      assert
        .dom('[data-test-invalid-path-error]')
        .exists('Space makes path invalid');
      assert
        .dom('[data-test-path-input]')
        .hasClass('hds-form-text-input--is-invalid');

      document.querySelector('[data-test-path-input]').value = '';
      await typeIn('[data-test-path-input]', '_');
      assert.dom('[data-test-invalid-path-error]').doesNotExist();

      const longString = 'a'.repeat(129);
      await typeIn('[data-test-path-input]', longString);
      assert
        .dom('[data-test-invalid-path-error]')
        .exists('Long name makes path invalid');
    });

    test('warns you when you set a key with . in it', async function (assert) {
      const mockedModel = this.server.create('variable', {
        keyValues: [{ key: '', value: '' }],
      });

      const testCases = [
        {
          name: 'valid key',
          key: 'superSecret2',
          warn: false,
        },
        {
          name: 'invalid key with dot',
          key: 'super.secret',
          warn: true,
        },
        {
          name: 'invalid key with slash',
          key: 'super/secret',
          warn: true,
        },
        {
          name: 'invalid key with emoji',
          key: 'supersecretspy🕵️',
          warn: true,
        },
        {
          name: 'unicode letters',
          key: '世界',
          warn: false,
        },
        {
          name: 'unicode numbers',
          key: '٣٢١',
          warn: false,
        },
        {
          name: 'unicode letters and numbers',
          key: '世٢界١',
          warn: false,
        },
      ];
      for (const testCase of testCases) {
        await render(
          <template><VariableForm @model={{mockedModel}} /></template>,
        );
        await typeIn('[data-test-var-key]', testCase.key);
        if (testCase.warn) {
          assert.dom('.key-value-error').exists(testCase.name);
        } else {
          assert.dom('.key-value-error').doesNotExist(testCase.name);
        }
      }
    });

    test('warns you when you create a duplicate key', async function (assert) {
      const mockedModel = this.server.create('variable', {
        keyValues: [{ key: 'myKey', value: 'myVal' }],
      });

      await render(
        <template><VariableForm @model={{mockedModel}} /></template>,
      );

      await click('[data-test-add-kv]');

      const secondKey = document.querySelectorAll('[data-test-var-key]')[1];
      await typeIn(secondKey, 'myWonderfulKey');
      assert.dom('.key-value-error').doesNotExist();

      secondKey.value = '';

      await typeIn(secondKey, 'myKey');
      assert.dom('.key-value-error').exists();
    });
  });

  module('Views', function () {
    test('Allows you to swap between JSON and Key/Value Views', async function (assert) {
      const state = new State();
      const mockedModel = this.server.create('variable', {
        path: '',
        keyValues: [{ key: '', value: '' }],
      });

      const existingVariables = this.server.createList('variable', 1, {
        path: 'baz/bat',
      });

      await render(
        <template>
          <VariableForm
            @model={{mockedModel}}
            @existingVariables={{existingVariables}}
            @view={{state.view}}
          />
        </template>,
      );
      assert.dom('.key-value').exists();
      assert.dom('.CodeMirror').doesNotExist();

      state.view = 'json';
      await settled();
      assert.dom('.key-value').doesNotExist();
      assert.dom('.CodeMirror').exists();
    });

    test('Persists Key/Values table data to JSON', async function (assert) {
      const state = new State();
      faker.seed(1);
      const keyValues = [
        { key: 'foo', value: '123' },
        { key: 'bar', value: '456' },
      ];
      const mockedModel = this.server.create('variable', {
        path: '',
        keyValues,
      });
      state.view = 'json';

      await render(
        <template>
          <VariableForm @model={{mockedModel}} @view={{state.view}} />
        </template>,
      );

      await percySnapshot(assert);

      const keyValuesAsJSON = keyValues.reduce(
        (accumulator, { key, value }) => {
          accumulator[key] = value;
          return accumulator;
        },
        {},
      );

      assert.deepEqual(
        code('.editor-wrapper').get(),
        JSON.stringify(keyValuesAsJSON, null, 2),
        'JSON editor contains the key values, stringified, by default',
      );

      state.view = 'table';
      await settled();

      await click('[data-test-add-kv]');

      const keyControl = findKeyControl();
      const valueControl = findValueControl();
      assert.ok(keyControl, 'Found key input control');
      assert.ok(valueControl, 'Found value input control');

      await typeIn(keyControl, 'howdy');
      await typeIn(valueControl, 'partner');
      await triggerEvent(keyControl, 'change');
      await triggerEvent(valueControl, 'change');
      await triggerEvent(keyControl, 'blur');
      await triggerEvent(valueControl, 'blur');

      state.view = 'json';
      await settled();

      const parsedJSON = JSON.parse(code('[data-test-json-editor]').get());
      const parsedObject = Array.isArray(parsedJSON)
        ? parsedJSON[0]
        : parsedJSON;

      assert.strictEqual(
        parsedObject?.howdy,
        'partner',
        'JSON editor contains the new key value',
      );
    });

    test('Persists JSON data to Key/Values table', async function (assert) {
      const state = new State();
      const keyValues = [{ key: '', value: '' }];
      const mockedModel = this.server.create('variable', {
        path: '',
        keyValues,
      });
      state.view = 'json';

      await render(
        <template>
          <VariableForm @model={{mockedModel}} @view={{state.view}} />
        </template>,
      );

      codeFillable('[data-test-json-editor]').get()(
        JSON.stringify({ golden: 'gate' }, null, 2),
      );
      state.view = 'table';
      await settled();
      assert.deepEqual(
        findKeyControl()?.value,
        'golden',
        'Key persists from JSON to Table',
      );

      assert.deepEqual(
        findValueControl()?.value,
        'gate',
        'Value persists from JSON to Table',
      );
    });
  });
});
