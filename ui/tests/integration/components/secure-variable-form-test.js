import { module, test } from 'qunit';
import { setupRenderingTest } from 'ember-qunit';
import { hbs } from 'ember-cli-htmlbars';
import { componentA11yAudit } from 'nomad-ui/tests/helpers/a11y-audit';
import { click, typeIn, findAll, render } from '@ember/test-helpers';
import { setupMirage } from 'ember-cli-mirage/test-support';

module('Integration | Component | secure-variable-form', function (hooks) {
  setupRenderingTest(hooks);
  setupMirage(hooks);

  test('passes an accessibility audit', async function (assert) {
    assert.expect(1);
    await render(hbs`<SecureVariableForm @model={{this.mockedModel}} />`);
    await componentA11yAudit(this.element, assert);
  });

  test('shows a single row by default and modifies on "Add More" and "Delete"', async function (assert) {
    this.set(
      'mockedModel',
      server.create('variable', {
        keyValues: [{ key: '', value: '' }],
      })
    );
    assert.expect(7);

    await render(hbs`<SecureVariableForm @model={{this.mockedModel}} />`);
    assert.equal(
      findAll('div.key-value').length,
      1,
      'A single KV row exists by default'
    );

    assert
      .dom('.key-value button.add-more')
      .isDisabled(
        'The "Add More" button is disabled until key and value are filled'
      );

    await typeIn('.key-value label:nth-child(1) input', 'foo');

    assert
      .dom('.key-value button.add-more')
      .isDisabled(
        'The "Add More" button is still disabled with only key filled'
      );

    await typeIn('.key-value label:nth-child(2) input', 'bar');

    assert
      .dom('.key-value button.add-more')
      .isNotDisabled(
        'The "Add More" button is no longer disabled after key and value are filled'
      );

    await click('.key-value button.add-more');

    assert.equal(
      findAll('div.key-value').length,
      2,
      'A second KV row exists after adding a new one'
    );

    await typeIn('.key-value:last-of-type label:nth-child(1) input', 'foo');
    await typeIn('.key-value:last-of-type label:nth-child(2) input', 'bar');

    await click('.key-value button.add-more');

    assert.equal(
      findAll('div.key-value').length,
      3,
      'A third KV row exists after adding a new one'
    );

    await click('.key-value button.delete-row');

    assert.equal(
      findAll('div.key-value').length,
      2,
      'Back down to two rows after hitting delete'
    );
  });

  test('Values can be toggled to show/hide', async function (assert) {
    this.set(
      'mockedModel',
      server.create('variable', {
        keyValues: [{ key: 'foo', value: 'bar' }],
      })
    );

    assert.expect(6);

    await render(hbs`<SecureVariableForm @model={{this.mockedModel}} />`);
    await click('.key-value button.add-more'); // add a second variable

    findAll('input.value-input').forEach((input, iter) => {
      assert.equal(
        input.getAttribute('type'),
        'password',
        `Value ${iter + 1} is hidden by default`
      );
    });

    await click('.key-value button.show-hide-values');
    findAll('input.value-input').forEach((input, iter) => {
      assert.equal(
        input.getAttribute('type'),
        'text',
        `Value ${iter + 1} is shown when toggled`
      );
    });

    await click('.key-value button.show-hide-values');
    findAll('input.value-input').forEach((input, iter) => {
      assert.equal(
        input.getAttribute('type'),
        'password',
        `Value ${iter + 1} is hidden when toggled again`
      );
    });
  });

  test('Existing variable shows properties by default', async function (assert) {
    assert.expect(13);
    const keyValues = [
      { key: 'my-completely-normal-key', value: 'never' },
      { key: 'another key, but with spaces', value: 'gonna' },
      { key: 'once/more/with/slashes', value: 'give' },
      { key: 'and_some_underscores', value: 'you' },
      { key: 'and\\now/for-something_completely@different', value: 'up' },
    ];

    this.set(
      'mockedModel',
      server.create('variable', {
        path: 'my/path/to',
        keyValues,
      })
    );
    await render(hbs`<SecureVariableForm @model={{this.mockedModel}} />`);
    assert.equal(
      findAll('div.key-value').length,
      5,
      'Shows 5 existing key values'
    );
    assert.equal(
      findAll('button.delete-row').length,
      4,
      'Shows "delete" for the first four rows'
    );
    assert.equal(
      findAll('button.add-more').length,
      1,
      'Shows "add more" only on the last row'
    );

    findAll('div.key-value').forEach((row, idx) => {
      assert.equal(
        row.querySelector(`label:nth-child(1) input`).value,
        keyValues[idx].key,
        `Key ${idx + 1} is correct`
      );

      assert.equal(
        row.querySelector(`label:nth-child(2) input`).value,
        keyValues[idx].value,
        keyValues[idx].value
      );
    });
  });

  test('Prevent editing path input on existing variables', async function (assert) {
    assert.expect(3);

    const variable = await this.server.create('variable', {
      name: 'foo',
      namespace: 'bar',
      path: '/baz/bat',
      keyValues: [{ key: '', value: '' }],
    });
    variable.isNew = false;
    this.set('variable', variable);
    await render(hbs`<SecureVariableForm @model={{this.variable}} />`);
    assert.dom('input.path-input').hasValue('/baz/bat', 'Path is set');
    assert
      .dom('input.path-input')
      .isDisabled('Existing variable is in disabled state');

    variable.isNew = true;
    variable.path = '';
    this.set('variable', variable);
    await render(hbs`<SecureVariableForm @model={{this.variable}} />`);
    assert
      .dom('input.path-input')
      .isNotDisabled('New variable is not in disabled state');
  });

  module('Validation', function () {
    test('warns when you try to create a path that already exists', async function (assert) {
      this.set(
        'mockedModel',
        server.create('variable', {
          path: '',
          keyValues: [{ key: '', value: '' }],
        })
      );

      this.set(
        'existingVariables',
        server.createList('variable', 1, {
          path: 'baz/bat',
        })
      );

      await render(
        hbs`<SecureVariableForm @model={{this.mockedModel}} @existingVariables={{this.existingVariables}} />`
      );
      await typeIn('.path-input', 'baz/bat');
      assert.dom('.duplicate-path-error').exists();
      assert.dom('.path-input').hasClass('error');

      await typeIn('.path-input', 'foo/bar');
      assert.dom('.duplicate-path-error').doesNotExist();
      assert.dom('.path-input').doesNotHaveClass('error');
    });
  });
});
