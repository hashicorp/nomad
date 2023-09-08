/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import {
  findAll,
  find,
  click,
  focus,
  render,
  triggerKeyEvent,
} from '@ember/test-helpers';
import { module, test } from 'qunit';
import { setupRenderingTest } from 'ember-qunit';
import sinon from 'sinon';
import hbs from 'htmlbars-inline-precompile';
import { componentA11yAudit } from 'nomad-ui/tests/helpers/a11y-audit';

const TAB = 9;
const ESC = 27;
const SPACE = 32;
const ARROW_UP = 38;
const ARROW_DOWN = 40;

module('Integration | Component | multi-select dropdown', function (hooks) {
  setupRenderingTest(hooks);

  const commonProperties = () => ({
    label: 'This is the dropdown label',
    selection: [],
    options: [
      { key: 'consul', label: 'Consul' },
      { key: 'nomad', label: 'Nomad' },
      { key: 'terraform', label: 'Terraform' },
      { key: 'packer', label: 'Packer' },
      { key: 'vagrant', label: 'Vagrant' },
      { key: 'vault', label: 'Vault' },
    ],
    onSelect: sinon.spy(),
  });

  const commonTemplate = hbs`
    <MultiSelectDropdown
      @label={{this.label}}
      @options={{this.options}}
      @selection={{this.selection}}
      @onSelect={{this.onSelect}} />
  `;

  test('component is initially closed', async function (assert) {
    assert.expect(4);

    const props = commonProperties();
    this.setProperties(props);
    await render(commonTemplate);

    assert.ok(find('.dropdown-trigger'), 'Trigger is shown');
    assert.equal(
      find('[data-test-dropdown-trigger]').textContent.trim(),
      props.label,
      'Trigger is appropriately labeled'
    );
    assert.notOk(
      find('[data-test-dropdown-options]'),
      'Options are not rendered'
    );

    await componentA11yAudit(this.element, assert);
  });

  test('component opens the options dropdown when clicked', async function (assert) {
    assert.expect(3);

    const props = commonProperties();
    this.setProperties(props);
    await render(commonTemplate);

    await click('[data-test-dropdown-trigger]');

    await assert.ok(
      find('[data-test-dropdown-options]'),
      'Options are shown now'
    );
    await componentA11yAudit(this.element, assert);

    await click('[data-test-dropdown-trigger]');

    assert.notOk(
      find('[data-test-dropdown-options]'),
      'Options are hidden after clicking again'
    );
  });

  test('all options are shown in the options dropdown, each with a checkbox input', async function (assert) {
    assert.expect(13);

    const props = commonProperties();
    this.setProperties(props);
    await render(commonTemplate);

    await click('[data-test-dropdown-trigger]');

    assert.equal(
      findAll('[data-test-dropdown-option]').length,
      props.options.length,
      'All options are shown'
    );
    findAll('[data-test-dropdown-option]').forEach((optionEl, index) => {
      const label = props.options[index].label;
      assert.equal(
        optionEl.textContent.trim(),
        label,
        `Correct label for ${label}`
      );
      assert.ok(
        optionEl.querySelector('input[type="checkbox"]'),
        'Option contains a checkbox'
      );
    });
  });

  test('onSelect gets called when an option is clicked', async function (assert) {
    const props = commonProperties();
    this.setProperties(props);
    await render(commonTemplate);

    await click('[data-test-dropdown-trigger]');
    await click('[data-test-dropdown-option] label');

    assert.ok(props.onSelect.called, 'onSelect was called');
    const newSelection = props.onSelect.getCall(0).args[0];
    assert.deepEqual(
      newSelection,
      [props.options[0].key],
      'onSelect was called with the first option key'
    );
  });

  test('the component trigger shows the selection count when there is a selection', async function (assert) {
    assert.expect(4);

    const props = commonProperties();
    props.selection = [props.options[0].key, props.options[1].key];
    this.setProperties(props);
    await render(commonTemplate);

    assert.ok(
      find('[data-test-dropdown-trigger] [data-test-dropdown-count]'),
      'The count is shown'
    );
    assert.equal(
      find('[data-test-dropdown-trigger] [data-test-dropdown-count]')
        .textContent,
      props.selection.length,
      'The count is accurate'
    );

    await componentA11yAudit(this.element, assert);

    await this.set('selection', []);

    assert.notOk(
      find('[data-test-dropdown-trigger] [data-test-dropdown-count]'),
      'The count is no longer shown when the selection is empty'
    );
  });

  test('pressing DOWN when the trigger has focus opens the options list', async function (assert) {
    const props = commonProperties();
    this.setProperties(props);
    await render(commonTemplate);

    await focus('[data-test-dropdown-trigger]');
    assert.notOk(
      find('[data-test-dropdown-options]'),
      'Options are not shown on focus'
    );
    await triggerKeyEvent('[data-test-dropdown-trigger]', 'keyup', ARROW_DOWN);
    assert.ok(find('[data-test-dropdown-options]'), 'Options are now shown');
    assert.equal(
      document.activeElement,
      find('[data-test-dropdown-trigger]'),
      'The dropdown trigger maintains focus'
    );
  });

  test('pressing DOWN when the trigger has focus and the options list is open focuses the first option', async function (assert) {
    const props = commonProperties();
    this.setProperties(props);
    await render(commonTemplate);

    await focus('[data-test-dropdown-trigger]');
    await triggerKeyEvent('[data-test-dropdown-trigger]', 'keyup', ARROW_DOWN);
    await triggerKeyEvent('[data-test-dropdown-trigger]', 'keyup', ARROW_DOWN);
    assert.equal(
      document.activeElement,
      find('[data-test-dropdown-option]'),
      'The first option now has focus'
    );
  });

  test('pressing TAB when the trigger has focus and the options list is open focuses the first option', async function (assert) {
    const props = commonProperties();
    this.setProperties(props);
    await render(commonTemplate);

    await focus('[data-test-dropdown-trigger]');
    await triggerKeyEvent('[data-test-dropdown-trigger]', 'keyup', ARROW_DOWN);
    await triggerKeyEvent('[data-test-dropdown-trigger]', 'keyup', TAB);
    assert.equal(
      document.activeElement,
      find('[data-test-dropdown-option]'),
      'The first option now has focus'
    );
  });

  test('pressing UP when the first list option is focused does nothing', async function (assert) {
    const props = commonProperties();
    this.setProperties(props);
    await render(commonTemplate);

    await click('[data-test-dropdown-trigger]');

    await focus('[data-test-dropdown-option]');
    await triggerKeyEvent('[data-test-dropdown-option]', 'keyup', ARROW_UP);
    assert.equal(
      document.activeElement,
      find('[data-test-dropdown-option]'),
      'The first option maintains focus'
    );
  });

  test('pressing DOWN when the a list option is focused moves focus to the next list option', async function (assert) {
    const props = commonProperties();
    this.setProperties(props);
    await render(commonTemplate);

    await click('[data-test-dropdown-trigger]');

    await focus('[data-test-dropdown-option]');
    await triggerKeyEvent('[data-test-dropdown-option]', 'keyup', ARROW_DOWN);
    assert.equal(
      document.activeElement,
      findAll('[data-test-dropdown-option]')[1],
      'The second option has focus'
    );
  });

  test('pressing DOWN when the last list option has focus does nothing', async function (assert) {
    assert.expect(6);

    const props = commonProperties();
    this.setProperties(props);
    await render(commonTemplate);

    await click('[data-test-dropdown-trigger]');

    await focus('[data-test-dropdown-option]');
    const optionEls = findAll('[data-test-dropdown-option]');
    const lastIndex = optionEls.length - 1;

    for (const [index, option] of optionEls.entries()) {
      await triggerKeyEvent(option, 'keyup', ARROW_DOWN);

      if (index < lastIndex) {
        /* eslint-disable-next-line qunit/no-conditional-assertions */
        assert.equal(
          document.activeElement,
          optionEls[index + 1],
          `Option ${index + 1} has focus`
        );
      }
    }

    await triggerKeyEvent(optionEls[lastIndex], 'keyup', ARROW_DOWN);
    assert.equal(
      document.activeElement,
      optionEls[lastIndex],
      `Option ${lastIndex} still has focus`
    );
  });

  test('onSelect gets called when pressing SPACE when a list option is focused', async function (assert) {
    const props = commonProperties();
    this.setProperties(props);
    await render(commonTemplate);

    await click('[data-test-dropdown-trigger]');

    await focus('[data-test-dropdown-option]');
    await triggerKeyEvent('[data-test-dropdown-option]', 'keyup', SPACE);

    assert.ok(props.onSelect.called, 'onSelect was called');
    const newSelection = props.onSelect.getCall(0).args[0];
    assert.deepEqual(
      newSelection,
      [props.options[0].key],
      'onSelect was called with the first option key'
    );
  });

  test('list options have a zero tabindex and are therefore sequentially navigable', async function (assert) {
    assert.expect(6);

    const props = commonProperties();
    this.setProperties(props);
    await render(commonTemplate);

    await click('[data-test-dropdown-trigger]');

    findAll('[data-test-dropdown-option]').forEach((option) => {
      assert.equal(
        parseInt(option.getAttribute('tabindex'), 10),
        0,
        'tabindex is zero'
      );
    });
  });

  test('the checkboxes inside list options have a negative tabindex and are therefore not sequentially navigable', async function (assert) {
    assert.expect(6);

    const props = commonProperties();
    this.setProperties(props);
    await render(commonTemplate);

    await click('[data-test-dropdown-trigger]');

    findAll('[data-test-dropdown-option]').forEach((option) => {
      assert.ok(
        parseInt(
          option
            .querySelector('input[type="checkbox"]')
            .getAttribute('tabindex'),
          10
        ) < 0,
        'tabindex is a negative value'
      );
    });
  });

  test('pressing ESC when the options list is open closes the list and returns focus to the dropdown trigger', async function (assert) {
    const props = commonProperties();
    this.setProperties(props);
    await render(commonTemplate);

    await focus('[data-test-dropdown-trigger]');
    await triggerKeyEvent('[data-test-dropdown-trigger]', 'keyup', ARROW_DOWN);
    await triggerKeyEvent('[data-test-dropdown-trigger]', 'keyup', ARROW_DOWN);
    await triggerKeyEvent('[data-test-dropdown-option]', 'keyup', ESC);

    assert.notOk(
      find('[data-test-dropdown-options]'),
      'The options list is hidden once more'
    );
    assert.equal(
      document.activeElement,
      find('[data-test-dropdown-trigger]'),
      'The trigger has focus'
    );
  });

  test('when there are no list options, an empty message is shown', async function (assert) {
    assert.expect(4);

    const props = commonProperties();
    props.options = [];
    this.setProperties(props);
    await render(commonTemplate);

    await click('[data-test-dropdown-trigger]');
    assert.ok(
      find('[data-test-dropdown-options]'),
      'The dropdown is still shown'
    );
    assert.ok(find('[data-test-dropdown-empty]'), 'The empty state is shown');
    assert.notOk(find('[data-test-dropdown-option]'), 'No options are shown');
    await componentA11yAudit(this.element, assert);
  });
});
